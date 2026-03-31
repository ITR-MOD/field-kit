package config

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// gameNames are the subfolder names we look for under steamapps/common/.
var gameNames = []string{"IntoTheRadius2", "IntoTheRadius"}

// FindSteamInstalls scans well-known Steam library locations and returns
// absolute paths to any detected Into the Radius installations.
func FindSteamInstalls() []string {
	var candidates []string

	for _, base := range steamLibraryRoots() {
		common := filepath.Join(base, "steamapps", "common")
		for _, name := range gameNames {
			p := filepath.Join(common, name)
			if dirExists(p) {
				candidates = append(candidates, p)
			}
		}
	}

	return dedupe(candidates)
}

// steamLibraryRoots returns all Steam library root directories to search.
// A "root" is the directory that contains a steamapps/ subdirectory.
func steamLibraryRoots() []string {
	var roots []string

	switch runtime.GOOS {
	case "windows":
		roots = append(roots, windowsSteamRoots()...)
	default:
		roots = append(roots, linuxSteamRoots()...)
	}

	return dedupe(roots)
}

// ─── Linux ────────────────────────────────────────────────────────────────────

func linuxSteamRoots() []string {
	home, _ := os.UserHomeDir()

	defaults := []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam", "root"),
	}

	var roots []string
	for _, d := range defaults {
		if dirExists(d) {
			roots = append(roots, d)
		}
	}

	// Parse libraryfolders.vdf for additional library paths.
	for _, d := range defaults {
		for _, vdfPath := range []string{
			filepath.Join(d, "steamapps", "libraryfolders.vdf"),
			filepath.Join(d, "config", "libraryfolders.vdf"),
		} {
			roots = append(roots, parseVDFLibraryPaths(vdfPath)...)
		}
	}

	// Scan /mnt/* and /media/$USER/* for SteamLibrary directories.
	for _, mountBase := range mountBases() {
		entries, _ := os.ReadDir(mountBase)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			candidate := filepath.Join(mountBase, e.Name(), "SteamLibrary")
			if dirExists(candidate) {
				roots = append(roots, candidate)
			}
			// Also check the mount point itself if it looks like a Steam root.
			direct := filepath.Join(mountBase, e.Name())
			if dirExists(filepath.Join(direct, "steamapps")) {
				roots = append(roots, direct)
			}
		}
	}

	return roots
}

func mountBases() []string {
	bases := []string{"/mnt"}
	home, _ := os.UserHomeDir()
	user := filepath.Base(home)
	bases = append(bases, "/media/"+user, "/media")
	return bases
}

// ─── Windows ─────────────────────────────────────────────────────────────────

func windowsSteamRoots() []string {
	var roots []string

	// Well-known default install paths.
	defaults := []string{
		`C:\Program Files (x86)\Steam`,
		`C:\Program Files\Steam`,
	}
	for _, d := range defaults {
		if dirExists(d) {
			roots = append(roots, d)
		}
	}

	// Scan all drive letters for SteamLibrary directories.
	for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		drive := string(letter) + `:\`
		if !dirExists(drive) {
			continue
		}
		for _, sub := range []string{"SteamLibrary", "Steam", "Games\\Steam"} {
			candidate := filepath.Join(drive, sub)
			if dirExists(candidate) {
				roots = append(roots, candidate)
			}
		}
	}

	// Parse libraryfolders.vdf from any found Steam root.
	for _, d := range roots {
		for _, vdfPath := range []string{
			filepath.Join(d, "steamapps", "libraryfolders.vdf"),
			filepath.Join(d, "config", "libraryfolders.vdf"),
		} {
			roots = append(roots, parseVDFLibraryPaths(vdfPath)...)
		}
	}

	return roots
}

// ─── VDF parser ───────────────────────────────────────────────────────────────

// parseVDFLibraryPaths extracts "path" values from a libraryfolders.vdf file.
// This is a minimal line-oriented parser — it does not implement the full VDF
// spec, but handles both the old (index-keyed) and new (numbered object) formats
// that Steam has used over the years.
func parseVDFLibraryPaths(vdfPath string) []string {
	f, err := os.Open(vdfPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var paths []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Match lines like:   "path"   "/some/path"
		if !strings.HasPrefix(line, `"path"`) {
			continue
		}
		parts := splitVDFKeyValue(line)
		if len(parts) == 2 {
			p := filepath.Clean(parts[1])
			if dirExists(p) {
				paths = append(paths, p)
			}
		}
	}
	return paths
}

// splitVDFKeyValue splits a VDF line like `"key"   "value"` into [key, value].
func splitVDFKeyValue(line string) []string {
	var parts []string
	inQuote := false
	var cur strings.Builder
	for _, r := range line {
		switch {
		case r == '"':
			if inQuote {
				parts = append(parts, cur.String())
				cur.Reset()
			}
			inQuote = !inQuote
		case inQuote:
			cur.WriteRune(r)
		}
	}
	return parts
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func dedupe(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := ss[:0]
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
