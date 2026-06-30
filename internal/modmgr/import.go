package modmgr

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ITR-MOD/field-kit/internal/config"
	"github.com/mholt/archives"
)

// Import imports a mod from a zip, rar, or 7z archive.
// It:
//  1. Copies the archive to the archives cache dir.
//  2. Extracts files into mods/{id}/files/.
//  3. Detects mod type(s) and saves metadata.
//
// Returns the new ModMeta on success.
func Import(archivePath string) (*ModMeta, error) {
	archiveName := filepath.Base(archivePath)
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

	// Copy archive to cache (preserve original extension).
	ext := filepath.Ext(archiveName)
	destArchive := filepath.Join(config.ArchivesDir(), id+ext)
	if err := copyFileOS(archivePath, destArchive); err != nil {
		_ = os.RemoveAll(modDir)
		return nil, fmt.Errorf("cache archive: %w", err)
	}

	// Extract archive (zip, rar, 7z, etc.).
	relFiles, err := extractArchive(destArchive, filesDir)
	if err != nil {
		_ = os.RemoveAll(modDir)
		_ = os.Remove(destArchive)
		return nil, fmt.Errorf("extract: %w", err)
	}

	modTypes := DetectTypes(relFiles)
	name := strings.TrimSuffix(archiveName, ext)

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

	if err := os.RemoveAll(modDir); err != nil {
		return fmt.Errorf("remove mod dir: %w", err)
	}

	// Remove any cached archive regardless of extension.
	for _, ext := range []string{".zip", ".rar", ".7z"} {
		_ = os.Remove(filepath.Join(config.ArchivesDir(), id+ext))
	}
	return nil
}

// extractArchive extracts any supported archive (zip, rar, 7z) to destDir
// and returns the list of relative file paths that were extracted.
func extractArchive(src, destDir string) ([]string, error) {
	f, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	format, stream, err := archives.Identify(context.Background(), filepath.Base(src), f)
	if err != nil {
		return nil, fmt.Errorf("identify archive format: %w", err)
	}

	ex, ok := format.(archives.Extractor)
	if !ok {
		return nil, fmt.Errorf("format %s does not support extraction", format.Extension())
	}

	var relPaths []string

	handler := func(_ context.Context, info archives.FileInfo) error {
		rel := filepath.ToSlash(info.NameInArchive)
		// Sanitise: prevent path traversal.
		if strings.Contains(rel, "..") {
			return nil
		}

		destPath := filepath.Join(destDir, rel)

		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		rc, err := info.Open()
		if err != nil {
			return fmt.Errorf("open %s: %w", rel, err)
		}
		defer rc.Close()

		out, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer out.Close()

		if _, err := io.Copy(out, rc); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}

		relPaths = append(relPaths, rel)
		return nil
	}

	// Some formats (e.g. RAR) only satisfy Extraction not Archival;
	// the Extractor interface covers both via Extract(ctx, reader, handler).
	if err := ex.Extract(context.Background(), stream, handler); err != nil {
		return nil, err
	}

	return relPaths, nil
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

// ImportFolder imports a mod from a directory by copying all files.
// Mirrors Import but skips the archive step.
func ImportFolder(folderPath string) (*ModMeta, error) {
	folderName := filepath.Base(folderPath)
	id := makeModID(folderName)

	if _, err := os.Stat(filepath.Join(config.ModsDir(), id)); err == nil {
		return nil, fmt.Errorf("mod %q already imported (id: %s)", folderName, id)
	}

	modDir := filepath.Join(config.ModsDir(), id)
	filesDir := filepath.Join(modDir, "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return nil, fmt.Errorf("create mod dir: %w", err)
	}

	relFiles, err := copyDirRecursive(folderPath, filesDir)
	if err != nil {
		_ = os.RemoveAll(modDir)
		return nil, fmt.Errorf("copy folder: %w", err)
	}

	modTypes := DetectTypes(relFiles)
	meta := &ModMeta{
		ID:          id,
		Name:        folderName,
		ArchiveName: "",
		ImportedAt:  time.Now(),
		Types:       modTypes,
		Files:       relFiles,
	}
	if err := SaveMeta(meta); err != nil {
		return nil, fmt.Errorf("save meta: %w", err)
	}
	return meta, nil
}

// copyDirRecursive walks src and copies all files into dst, returning relative paths.
func copyDirRecursive(src, dst string) ([]string, error) {
	var relFiles []string
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// Prevent path traversal.
		if strings.Contains(filepath.ToSlash(rel), "..") {
			return nil
		}
		dest := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		if err := copyFileOS(path, dest); err != nil {
			return err
		}
		relFiles = append(relFiles, filepath.ToSlash(rel))
		return nil
	})
	return relFiles, err
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
