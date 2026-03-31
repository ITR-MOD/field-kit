// Package deploy handles symlinking mod files into the game directory,
// including backup/restore of original game files for "custom" placements.
package deploy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Merith-TK/itr-field-kit/internal/config"
	"github.com/Merith-TK/itr-field-kit/internal/modmgr"
)

// ─── Deployment state ────────────────────────────────────────────────────────

// DeployedFile records a single symlink that was created during deployment.
type DeployedFile struct {
	// GamePath is the absolute path to the symlink in the game directory.
	GamePath string `json:"game_path"`
	// ModID is the mod that owns this file.
	ModID string `json:"mod_id"`
	// IsCustom indicates whether this file required a backup before symlinking.
	IsCustom bool `json:"is_custom,omitempty"`
}

// State is the persisted deployment record.
type State struct {
	GameID   string         `json:"game_id"`
	Profile  string         `json:"profile"`
	Deployed []DeployedFile `json:"deployed"`
}

func loadState() (*State, error) {
	data, err := os.ReadFile(config.StatePath())
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	return &s, json.Unmarshal(data, &s)
}

func saveState(s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.StatePath(), data, 0644)
}

// ─── Backup registry ─────────────────────────────────────────────────────────

// BackupEntry records one backed-up game file.
type BackupEntry struct {
	// RelPath is the path relative to game root.
	RelPath    string `json:"rel_path"`
	// BackupFile is the filename (not path) within the game's backup dir.
	BackupFile string `json:"backup_file"`
}

type backupRegistry struct {
	Entries []BackupEntry `json:"entries"`
}

func backupRegistryPath(gameID string) string {
	return filepath.Join(config.GameBackupDir(gameID), "registry.json")
}

func loadBackupRegistry(gameID string) (*backupRegistry, error) {
	data, err := os.ReadFile(backupRegistryPath(gameID))
	if os.IsNotExist(err) {
		return &backupRegistry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var r backupRegistry
	return &r, json.Unmarshal(data, &r)
}

func saveBackupRegistry(gameID string, r *backupRegistry) error {
	if err := os.MkdirAll(config.GameBackupDir(gameID), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(backupRegistryPath(gameID), data, 0644)
}

// ─── Public API ───────────────────────────────────────────────────────────────

// Deploy applies a profile to a game installation.
// For each enabled mod (in order) it:
//  1. Maps mod files to game-root-relative destinations via the vortex logic.
//  2. For "custom" files that overwrite an existing regular game file, backs up the original.
//  3. Creates a symlink: gameRoot/dest → modCache/files/source.
//
// Deployment is idempotent for files that are already correctly symlinked.
func Deploy(game *config.GameInstall, profile *modmgr.Profile) error {
	state := &State{
		GameID:  game.ID,
		Profile: profile.Name,
	}

	backupReg, err := loadBackupRegistry(game.ID)
	if err != nil {
		return fmt.Errorf("load backup registry: %w", err)
	}

	for _, modID := range profile.Mods {
		meta, err := modmgr.LoadMeta(modID)
		if err != nil {
			return fmt.Errorf("load mod %q: %w", modID, err)
		}

		instructions := modmgr.MapFiles(meta.Files)

		for _, instr := range instructions {
			srcAbs := filepath.Join(meta.FilesDir(), filepath.FromSlash(instr.Source))
			dstAbs := filepath.Join(game.Path, filepath.FromSlash(instr.Dest))

			// Skip instructions whose source doesn't exist (e.g. partial packs).
			if _, err := os.Lstat(srcAbs); err != nil {
				continue
			}

			// Ensure parent directory exists.
			if err := os.MkdirAll(filepath.Dir(dstAbs), 0755); err != nil {
				return fmt.Errorf("mkdir for %s: %w", dstAbs, err)
			}

			// Handle existing destination.
			dstInfo, dstErr := os.Lstat(dstAbs)
			if dstErr == nil {
				if isSymlink(dstInfo) {
					// Already a symlink – check if it already points to our source.
					target, _ := os.Readlink(dstAbs)
					if target == srcAbs {
						// Already correctly deployed; record and continue.
						state.Deployed = append(state.Deployed, DeployedFile{
							GamePath: dstAbs, ModID: modID, IsCustom: instr.IsCustom,
						})
						continue
					}
					// Different symlink – remove and replace.
					if err := os.Remove(dstAbs); err != nil {
						return fmt.Errorf("remove old symlink %s: %w", dstAbs, err)
					}
				} else {
					// Regular file – back it up before replacing.
					if instr.IsCustom {
						if err := backupFile(game.ID, game.Path, instr.Dest, backupReg); err != nil {
							return fmt.Errorf("backup %s: %w", instr.Dest, err)
						}
					}
					if err := os.Remove(dstAbs); err != nil {
						return fmt.Errorf("remove game file %s: %w", dstAbs, err)
					}
				}
			}

			// Create the symlink.
			if err := createSymlink(srcAbs, dstAbs); err != nil {
				return fmt.Errorf("symlink %s → %s: %w", dstAbs, srcAbs, err)
			}

			state.Deployed = append(state.Deployed, DeployedFile{
				GamePath: dstAbs, ModID: modID, IsCustom: instr.IsCustom,
			})
		}
	}

	if err := saveBackupRegistry(game.ID, backupReg); err != nil {
		return fmt.Errorf("save backup registry: %w", err)
	}
	return saveState(state)
}

// Undeploy removes all symlinks recorded in the deployment state and restores
// any backed-up original game files.
func Undeploy(game *config.GameInstall) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	backupReg, err := loadBackupRegistry(game.ID)
	if err != nil {
		return fmt.Errorf("load backup registry: %w", err)
	}

	for _, df := range state.Deployed {
		info, err := os.Lstat(df.GamePath)
		if err != nil {
			// File already gone – fine.
			continue
		}
		if !isSymlink(info) {
			continue
		}
		if err := os.Remove(df.GamePath); err != nil {
			return fmt.Errorf("remove symlink %s: %w", df.GamePath, err)
		}

		// Restore backup if one exists for this path.
		if df.IsCustom {
			rel, err := filepath.Rel(game.Path, df.GamePath)
			if err == nil {
				_ = restoreBackup(game.ID, game.Path, filepath.ToSlash(rel), backupReg)
			}
		}
	}

	// Clear deployment state.
	return saveState(&State{})
}

// Status returns all deployed files.
func Status() (*State, error) {
	return loadState()
}

// ─── Backup helpers ───────────────────────────────────────────────────────────

// backupFile copies gameRoot/relPath → backupDir/{encoded-relPath}.bak
// and records the entry in the registry.
// If an entry already exists for this relPath the backup is refreshed
// (the file may have been updated by a game patch).
func backupFile(gameID, gameRoot, relPath string, reg *backupRegistry) error {
	srcAbs := filepath.Join(gameRoot, filepath.FromSlash(relPath))

	// Derive a flat filename safe for the backup directory.
	backupFilename := filepath.FromSlash(relPath)
	backupFilename = sanitizePath(backupFilename) + ".bak"

	backupAbs := filepath.Join(config.GameBackupDir(gameID), backupFilename)
	if err := os.MkdirAll(filepath.Dir(backupAbs), 0755); err != nil {
		return err
	}

	if err := copyFileOS(srcAbs, backupAbs); err != nil {
		return err
	}

	// Upsert registry entry.
	for i, e := range reg.Entries {
		if e.RelPath == relPath {
			reg.Entries[i].BackupFile = backupFilename
			return nil
		}
	}
	reg.Entries = append(reg.Entries, BackupEntry{RelPath: relPath, BackupFile: backupFilename})
	return nil
}

// restoreBackup copies a backup file back to gameRoot/relPath.
func restoreBackup(gameID, gameRoot, relPath string, reg *backupRegistry) error {
	for _, e := range reg.Entries {
		if e.RelPath != relPath {
			continue
		}
		backupAbs := filepath.Join(config.GameBackupDir(gameID), e.BackupFile)
		dstAbs := filepath.Join(gameRoot, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(dstAbs), 0755); err != nil {
			return err
		}
		return copyFileOS(backupAbs, dstAbs)
	}
	return nil // no backup – nothing to restore
}

// ─── OS helpers ───────────────────────────────────────────────────────────────

func isSymlink(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}

// createSymlink creates a symlink, using an absolute target path.
// On Windows it tries a junction for directories first as a fallback when
// developer mode / admin rights aren't available (best-effort).
func createSymlink(target, link string) error {
	if runtime.GOOS == "windows" {
		// Try regular symlink first; it works in developer mode.
		err := os.Symlink(target, link)
		if err != nil {
			return fmt.Errorf("symlink (requires developer mode or admin on Windows): %w", err)
		}
		return nil
	}
	return os.Symlink(target, link)
}

func copyFileOS(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// sanitizePath converts a relative path to a flat safe filename.
func sanitizePath(p string) string {
	result := make([]byte, 0, len(p))
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '/' || c == '\\' || c == ':' {
			result = append(result, '_')
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}
