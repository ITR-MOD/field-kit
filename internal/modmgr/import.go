package modmgr

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ITR-MOD/field-kit/internal/config"
)

// Import imports a mod from a zip archive.
// It:
//  1. Copies the archive to the archives cache dir.
//  2. Extracts files into mods/{id}/files/.
//  3. Detects mod type(s) and saves metadata.
//
// Returns the new ModMeta on success.
func Import(zipPath string) (*ModMeta, error) {
	archiveName := filepath.Base(zipPath)
	id := makeModID(archiveName)

	// Prevent duplicate imports by ID.
	if _, err := os.Stat(filepath.Join(config.ModsDir(), id)); err == nil {
		return nil, fmt.Errorf("mod %q already imported (id: %s)", archiveName, id)
	}

	// Ensure destination dirs exist.
	modDir := filepath.Join(config.ModsDir(), id)
	filesDir := filepath.Join(modDir, "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return nil, fmt.Errorf("create mod dir: %w", err)
	}

	// Copy archive to cache.
	destArchive := filepath.Join(config.ArchivesDir(), id+".zip")
	if err := copyFileOS(zipPath, destArchive); err != nil {
		_ = os.RemoveAll(modDir)
		return nil, fmt.Errorf("cache archive: %w", err)
	}

	// Extract zip.
	relFiles, err := extractZip(destArchive, filesDir)
	if err != nil {
		_ = os.RemoveAll(modDir)
		_ = os.Remove(destArchive)
		return nil, fmt.Errorf("extract: %w", err)
	}

	modTypes := DetectTypes(relFiles)
	name := strings.TrimSuffix(archiveName, filepath.Ext(archiveName))

	meta := &ModMeta{
		ID:          id,
		Name:        name,
		ArchiveName: archiveName,
		ImportedAt:  time.Now(),
		Types:       modTypes,
		Files:       relFiles,
	}
	if err := SaveMeta(meta); err != nil {
		return nil, fmt.Errorf("save meta: %w", err)
	}
	return meta, nil
}

// RemoveMod deletes all cached files for a mod.
func RemoveMod(id string) error {
	modDir := filepath.Join(config.ModsDir(), id)
	archivePath := filepath.Join(config.ArchivesDir(), id+".zip")

	if err := os.RemoveAll(modDir); err != nil {
		return fmt.Errorf("remove mod dir: %w", err)
	}
	// Archive removal is best-effort.
	_ = os.Remove(archivePath)
	return nil
}

// extractZip extracts a zip archive to destDir and returns the list of
// relative file paths that were extracted.
func extractZip(zipPath, destDir string) ([]string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var relPaths []string

	for _, f := range r.File {
		// Sanitise path to prevent zip-slip.
		rel := filepath.ToSlash(f.Name)
		if strings.Contains(rel, "..") {
			continue
		}

		destPath := filepath.Join(destDir, rel)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return nil, err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, err
		}

		if err := extractFile(f, destPath); err != nil {
			return nil, fmt.Errorf("extract %s: %w", rel, err)
		}
		relPaths = append(relPaths, rel)
	}
	return relPaths, nil
}

func extractFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// copyFileOS is a simple file copy helper.
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

// makeModID derives a filesystem-safe unique ID from the archive filename.
// Strips the extension and sanitises special characters.
func makeModID(archiveName string) string {
	name := strings.TrimSuffix(archiveName, filepath.Ext(archiveName))
	replacer := strings.NewReplacer(
		" ", "_", "/", "_", "\\", "_", ":", "_",
		"*", "_", "?", "_", "\"", "_", "<", "_",
		">", "_", "|", "_",
	)
	return replacer.Replace(name)
}
