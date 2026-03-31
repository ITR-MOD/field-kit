package modmgr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ITR-MOD/field-kit/internal/config"
)

// ModType describes which installer type was detected for a mod.
type ModType string

const (
	ModTypeUE4SS   ModType = "ue4ss"
	ModTypeLua     ModType = "lua"
	ModTypeShared  ModType = "shared"
	ModTypePak     ModType = "pak"
	ModTypeLogic   ModType = "logicmod"
	ModTypeCustom  ModType = "custom"
	ModTypeUnknown ModType = "unknown"
)

// ModMeta holds persisted metadata for an imported mod.
type ModMeta struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ArchiveName string    `json:"archive_name"`
	ImportedAt  time.Time `json:"imported_at"`
	Types       []ModType `json:"types"`
	Files       []string  `json:"files"` // relative paths within files/
}

// FilesDir returns the absolute path to the extracted files for this mod.
func (m *ModMeta) FilesDir() string {
	return filepath.Join(config.ModsDir(), m.ID, "files")
}

// MetaPath returns the path to the mod's meta.json.
func MetaPath(id string) string {
	return filepath.Join(config.ModsDir(), id, "meta.json")
}

// LoadMeta loads a mod's metadata from disk.
func LoadMeta(id string) (*ModMeta, error) {
	data, err := os.ReadFile(MetaPath(id))
	if err != nil {
		return nil, fmt.Errorf("load mod %q: %w", id, err)
	}
	var m ModMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse mod %q: %w", id, err)
	}
	return &m, nil
}

// SaveMeta persists a mod's metadata to disk.
func SaveMeta(m *ModMeta) error {
	dir := filepath.Join(config.ModsDir(), m.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(MetaPath(m.ID), data, 0644)
}

// ListMods returns all imported mod IDs by scanning the mods directory.
func ListMods() ([]*ModMeta, error) {
	entries, err := os.ReadDir(config.ModsDir())
	if err != nil {
		return nil, err
	}
	var mods []*ModMeta
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "archives" {
			continue
		}
		m, err := LoadMeta(e.Name())
		if err != nil {
			continue // skip corrupted entries
		}
		mods = append(mods, m)
	}
	return mods, nil
}

// Profile holds the list of enabled mods for a named profile.
type Profile struct {
	Name      string                  `json:"name"`
	Mods      []string                `json:"mods"`                // ordered list of mod IDs (earlier = lower priority)
	Overrides map[string]ModOverrides `json:"overrides,omitempty"` // per-mod destination overrides; key = modID
}

// ProfilePath returns the path for a profile's JSON file.
func ProfilePath(name string) string {
	return filepath.Join(config.ProfilesDir(), name+".json")
}

// LoadProfile loads a profile by name, creating a default empty one if not found.
func LoadProfile(name string) (*Profile, error) {
	data, err := os.ReadFile(ProfilePath(name))
	if os.IsNotExist(err) {
		return &Profile{Name: name}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load profile %q: %w", name, err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse profile %q: %w", name, err)
	}
	return &p, nil
}

// Save persists a profile to disk.
func (p *Profile) Save() error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ProfilePath(p.Name), data, 0644)
}

// HasMod reports whether mod id is in the profile.
func (p *Profile) HasMod(id string) bool {
	for _, m := range p.Mods {
		if m == id {
			return true
		}
	}
	return false
}

// AddMod appends mod id if not already present.
func (p *Profile) AddMod(id string) {
	if !p.HasMod(id) {
		p.Mods = append(p.Mods, id)
	}
}

// RemoveMod removes mod id from the profile.
func (p *Profile) RemoveMod(id string) {
	filtered := p.Mods[:0]
	for _, m := range p.Mods {
		if m != id {
			filtered = append(filtered, m)
		}
	}
	p.Mods = filtered
}

// ListProfiles returns names of all saved profiles.
func ListProfiles() ([]string, error) {
	entries, err := os.ReadDir(config.ProfilesDir())
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name()[:len(e.Name())-5])
		}
	}
	return names, nil
}
