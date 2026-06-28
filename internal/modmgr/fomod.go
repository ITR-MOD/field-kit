package modmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ITR-MOD/field-kit/internal/fomod"
)

// findFomodConfig locates a mod's fomod/ModuleConfig.xml among its extracted
// files and returns its relative path plus the "fomod root" — the directory
// containing the fomod/ folder. Source paths inside ModuleConfig.xml are
// relative to that root, not necessarily to the mod's files/ directory
// (archives commonly wrap everything in a single top-level folder).
func findFomodConfig(files []string) (relPath, root string, ok bool) {
	for _, f := range files {
		if strings.EqualFold(base(f), "ModuleConfig.xml") && strings.EqualFold(base(dir(f)), "fomod") {
			root = dir(dir(f))
			if root == "." {
				root = ""
			}
			return f, root, true
		}
	}
	return "", "", false
}

// LoadFomodConfig reads and parses a FOMOD mod's ModuleConfig.xml.
func LoadFomodConfig(modID string) (*fomod.Config, error) {
	meta, err := LoadMeta(modID)
	if err != nil {
		return nil, err
	}
	relPath, _, ok := findFomodConfig(meta.Files)
	if !ok {
		return nil, fmt.Errorf("mod %q has no fomod/ModuleConfig.xml", modID)
	}
	data, err := os.ReadFile(filepath.Join(meta.FilesDir(), filepath.FromSlash(relPath)))
	if err != nil {
		return nil, fmt.Errorf("read ModuleConfig.xml for %q: %w", modID, err)
	}
	return fomod.Parse(data)
}

// ConvertFomodMappings resolves a wizard's finalized fomod.FileMapping list
// into FomodFileEntry values whose Source is relative to the mod's files/
// directory — i.e. it accounts for the fomod root offset described in
// findFomodConfig.
func ConvertFomodMappings(modID string, files []fomod.FileMapping) ([]FomodFileEntry, error) {
	meta, err := LoadMeta(modID)
	if err != nil {
		return nil, err
	}
	_, root, ok := findFomodConfig(meta.Files)
	if !ok {
		return nil, fmt.Errorf("mod %q has no fomod/ModuleConfig.xml", modID)
	}

	entries := make([]FomodFileEntry, 0, len(files))
	for _, f := range files {
		src := filepath.ToSlash(f.Source)
		if root != "" {
			src = filepath.ToSlash(filepath.Join(root, src))
		}
		entries = append(entries, FomodFileEntry{Source: src, Dest: f.Dest})
	}
	return entries, nil
}

// SaveFomodSelection persists the resolved file list from a completed FOMOD
// install wizard into the mod's metadata (the mod's default selection).
func SaveFomodSelection(modID string, files []fomod.FileMapping) error {
	entries, err := ConvertFomodMappings(modID, files)
	if err != nil {
		return err
	}
	meta, err := LoadMeta(modID)
	if err != nil {
		return err
	}
	meta.FomodFiles = entries
	return SaveMeta(meta)
}
