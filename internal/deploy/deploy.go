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
	"strings"

	"github.com/ITR-MOD/field-kit/internal/config"
	"github.com/ITR-MOD/field-kit/internal/modmgr"
)

// ─── Deployment state ────────────────────────────────────────────────────────

// DeployedFile records a single file placed during deployment.
type DeployedFile struct {
	// GamePath is the absolute path to the file/symlink in the game directory.
	GamePath string `json:"game_path"`
	// ModID is the mod that owns this file.
	ModID string `json:"mod_id"`
	// IsCustom indicates whether this file required a backup before symlinking.
	IsCustom bool `json:"is_custom,omitempty"`
	// IsWriteFile indicates this was a plain write (not a symlink) of
	// manager-provided content such as override.txt.
	IsWriteFile bool `json:"is_write_file,omitempty"`
}

// State is the persisted deployment record (legacy; kept for migration fallback).
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

// GetState returns the current deployment state from the app-data record.
func GetState() (*State, error) {
	return loadState()
}

func saveState(s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.StatePath(), data, 0644)
}

// ─── Deployment manifest (written to game directory) ─────────────────────────

const manifestFilename = "field-kit-manifest.json"

// DeploymentManifest is the source-of-truth file written into the game root.
// It records exactly what field-kit has deployed so that deploy/undeploy can
// detect profile switches, clean up orphaned files, and operate correctly even
// when the app-data state.json is missing or stale.
type DeploymentManifest struct {
	// ProfileID is the name of the profile that produced this deployment.
	// Will become a UUID once profile sharing is implemented.
	ProfileID string `json:"profile_id"`
	// InstallationID is the GameInstall.ID hash for the game this was deployed to.
	InstallationID string `json:"installation_id"`
	// DeployedFiles is the complete list of every file placed during deployment.
	DeployedFiles []DeployedFile `json:"deployed_files"`
}

func manifestPath(gamePath string) string {
	return filepath.Join(gamePath, manifestFilename)
}

// loadManifest reads the manifest from the game directory.
// Returns nil (not an error) if the file does not exist.
func loadManifest(gamePath string) (*DeploymentManifest, error) {
	data, err := os.ReadFile(manifestPath(gamePath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m DeploymentManifest
	return &m, json.Unmarshal(data, &m)
}

func saveManifest(gamePath string, m *DeploymentManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath(gamePath), data, 0644)
}

func deleteManifest(gamePath string) error {
	err := os.Remove(manifestPath(gamePath))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ─── Backup registry ─────────────────────────────────────────────────────────

// BackupEntry records one backed-up game file.
type BackupEntry struct {
	// RelPath is the path relative to game root.
	RelPath string `json:"rel_path"`
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
// On every run it reads the existing manifest from the game directory to:
//   - Fully wipe a previous deployment when the profile or installation has changed.
//   - Remove orphaned files (from mods removed from the profile) when the profile is the same.
func Deploy(game *config.GameInstall, profile *modmgr.Profile) error {
	backupReg, err := loadBackupRegistry(game.ID)
	if err != nil {
		return fmt.Errorf("load backup registry: %w", err)
	}

	// ── Manifest-aware pre-deploy cleanup ────────────────────────────────────
	oldManifest, err := loadManifest(game.Path)
	if err != nil {
		return fmt.Errorf("load deployment manifest: %w", err)
	}

	// No manifest found: fall back to legacy state.json so that orphan detection
	// and profile-change detection work correctly for deployments made before
	// manifest support was added (migration path).
	if oldManifest == nil {
		state, err := loadState()
		if err == nil && len(state.Deployed) > 0 {
			oldManifest = &DeploymentManifest{
				ProfileID:      state.Profile,
				InstallationID: state.GameID,
				DeployedFiles:  state.Deployed,
			}
		}
	}

	if oldManifest != nil {
		profileChanged := oldManifest.ProfileID != profile.Name || oldManifest.InstallationID != game.ID
		if profileChanged {
			// Different profile or game install: remove every previously deployed file.
			if err := removeDeployedFiles(game, oldManifest.DeployedFiles, backupReg); err != nil {
				return fmt.Errorf("wipe old deployment: %w", err)
			}
		}
		// Orphan cleanup for same-profile re-deploys happens after we know the
		// new file set; see end of function.
	}

	// newFiles accumulates every file placed in this deploy run.
	var newFiles []DeployedFile

	for _, modID := range profile.Mods {
		meta, err := modmgr.LoadMeta(modID)
		if err != nil {
			return fmt.Errorf("load mod %q: %w", modID, err)
		}

		var instructions []modmgr.Instruction
		if meta.IsFomod() {
			entries := meta.FomodFiles
			if profEntries := profile.FomodEntriesFor(modID); len(profEntries) > 0 {
				entries = profEntries
			}
			if len(entries) == 0 {
				return fmt.Errorf("mod %q needs FOMOD setup before it can be deployed — configure its default in the Mods tab", meta.Name)
			}
			instructions = modmgr.MapFomodEntries(entries)
		} else {
			instructions = modmgr.MapFilesForGame(meta.Files, config.GameInternalID(game.Version))
		}

		// Apply destination overrides: profile > mod > default.
		modOvr, _ := modmgr.LoadModOverrides(modID)
		for i, instr := range instructions {
			src := instr.Source
			// 1. Profile-level override — highest priority; short-circuits mod-level.
			if profile.Overrides != nil {
				if profMod, ok := profile.Overrides[modID]; ok {
					if dest, ok := profMod.Sources[src]; ok {
						instructions[i].Dest = dest
						continue
					}
				}
			}
			// 2. Mod-level override — only if profile didn't override this source.
			if modOvr != nil {
				if dest, ok := modOvr.Sources[src]; ok {
					instructions[i].Dest = dest
				}
			}
		}

		for _, instr := range instructions {
			dstAbs := filepath.Join(game.Path, filepath.FromSlash(instr.Dest))

			// Safety: never touch protected game-root files.
			if isDeniedDest(game.Path, dstAbs) {
				continue
			}

			// ── WriteContent: manager-provided file (e.g. override.txt) ─────
			// Write a fixed string to dstAbs as a plain file; do not symlink.
			if instr.WriteContent != "" {
				if err := os.MkdirAll(filepath.Dir(dstAbs), 0755); err != nil {
					return fmt.Errorf("mkdir for %s: %w", dstAbs, err)
				}
				if err := os.WriteFile(dstAbs, []byte(instr.WriteContent), 0644); err != nil {
					return fmt.Errorf("write %s: %w", dstAbs, err)
				}
				newFiles = append(newFiles, DeployedFile{
					GamePath:    dstAbs,
					ModID:       modID,
					IsWriteFile: true,
				})
				continue
			}

			srcAbs := filepath.Join(meta.FilesDir(), filepath.FromSlash(instr.Source))

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
						newFiles = append(newFiles, DeployedFile{
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

			newFiles = append(newFiles, DeployedFile{
				GamePath: dstAbs, ModID: modID, IsCustom: instr.IsCustom,
			})
		}
	}

	// ── Same-profile orphan cleanup ───────────────────────────────────────────
	// Find files from the previous deployment that are no longer in the new set
	// (i.e. their mod was removed from the profile) and remove/restore them.
	if oldManifest != nil && oldManifest.ProfileID == profile.Name && oldManifest.InstallationID == game.ID {
		newSet := make(map[string]string, len(newFiles)) // GamePath → ModID
		for _, f := range newFiles {
			newSet[f.GamePath] = f.ModID
		}
		var orphans []DeployedFile
		for _, f := range oldManifest.DeployedFiles {
			if _, stillPresent := newSet[f.GamePath]; !stillPresent {
				orphans = append(orphans, f)
			}
		}
		if err := removeDeployedFiles(game, orphans, backupReg); err != nil {
			return fmt.Errorf("remove orphaned files: %w", err)
		}
	}

	if err := saveBackupRegistry(game.ID, backupReg); err != nil {
		return fmt.Errorf("save backup registry: %w", err)
	}

	// Write manifest to game directory.
	manifest := &DeploymentManifest{
		ProfileID:      profile.Name,
		InstallationID: game.ID,
		DeployedFiles:  newFiles,
	}
	if err := saveManifest(game.Path, manifest); err != nil {
		return fmt.Errorf("save deployment manifest: %w", err)
	}

	// Keep state.json in sync for any tooling that still reads it.
	return saveState(&State{GameID: game.ID, Profile: profile.Name, Deployed: newFiles})
}

// isDeniedDest reports whether deploying to dstAbs is forbidden.
// Denied targets (relative to game root):
//   - The deployment manifest itself (field-kit-manifest.json)
//   - Any IntoTheRadius*.exe directly in the game root
func isDeniedDest(gamePath, dstAbs string) bool {
	rel, err := filepath.Rel(gamePath, dstAbs)
	if err != nil {
		return false
	}
	// Reject paths that escape the game root.
	if strings.HasPrefix(rel, "..") {
		return true
	}
	// Only check files directly in the game root (no sub-directory separator).
	if filepath.Dir(rel) == "." {
		name := filepath.Base(rel)
		if name == manifestFilename {
			return true
		}
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "intotheradius") && strings.HasSuffix(lower, ".exe") {
			return true
		}
	}
	return false
}

func removeDeployedFiles(game *config.GameInstall, files []DeployedFile, backupReg *backupRegistry) error {
	for _, df := range files {
		if _, err := os.Lstat(df.GamePath); err != nil {
			// File already gone – fine.
			continue
		}

		if df.IsWriteFile {
			_ = os.Remove(df.GamePath)
			continue
		}

		info, err := os.Lstat(df.GamePath)
		if err != nil {
			continue
		}
		if !isSymlink(info) {
			continue
		}
		if err := os.Remove(df.GamePath); err != nil {
			return fmt.Errorf("remove symlink %s: %w", df.GamePath, err)
		}

		if df.IsCustom {
			rel, err := filepath.Rel(game.Path, df.GamePath)
			if err == nil {
				_ = restoreBackup(game.ID, game.Path, filepath.ToSlash(rel), backupReg)
			}
		}
	}
	return nil
}

// Undeploy removes all symlinks recorded in the deployment manifest (or legacy
// state.json) and restores any backed-up original game files.
func Undeploy(game *config.GameInstall) error {
	backupReg, err := loadBackupRegistry(game.ID)
	if err != nil {
		return fmt.Errorf("load backup registry: %w", err)
	}

	// Primary source: manifest in game directory.
	manifest, err := loadManifest(game.Path)
	if err != nil {
		return fmt.Errorf("load deployment manifest: %w", err)
	}

	var deployed []DeployedFile
	if manifest != nil {
		deployed = manifest.DeployedFiles
	} else {
		// Fallback: legacy state.json for deployments made before manifest support.
		state, err := loadState()
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}
		deployed = state.Deployed
	}

	if err := removeDeployedFiles(game, deployed, backupReg); err != nil {
		return err
	}

	// Clear deployment state.
	if err := deleteManifest(game.Path); err != nil {
		return fmt.Errorf("delete deployment manifest: %w", err)
	}
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
