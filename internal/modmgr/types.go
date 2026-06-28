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
	ModTypeSML     ModType = "sml"
	ModTypeFomod   ModType = "fomod"
	ModTypeUnknown ModType = "unknown"
)

// FomodFileEntry is one resolved source -> destination pair chosen by the
// user while running the FOMOD install wizard. Source is relative to the
// mod's files/ directory; Dest is relative to the game root.
type FomodFileEntry struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

// ModMeta holds persisted metadata for an imported mod.
type ModMeta struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ArchiveName string    `json:"archive_name"`
	ImportedAt  time.Time `json:"imported_at"`
	Types       []ModType `json:"types"`
	Files       []string  `json:"files"` // relative paths within files/

	// FomodFiles holds the resolved file list once a ModTypeFomod mod's
	// install wizard has been completed. Nil/empty means setup is pending.
	FomodFiles []FomodFileEntry `json:"fomod_files,omitempty"`
}

// IsFomod reports whether this mod is a FOMOD installer, regardless of
// whether its default selection has been configured yet.
func (m *ModMeta) IsFomod() bool {
	for _, t := range m.Types {
		if t == ModTypeFomod {
			return true
		}
	}
	return false
}

// IsFomodPending reports whether this mod is a FOMOD installer whose wizard
// has not yet been completed.
func (m *ModMeta) IsFomodPending() bool {
	return m.IsFomod() && len(m.FomodFiles) == 0
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
	GameID    string                  `json:"game_id"`
	Name      string                  `json:"name"`
	Mods      []string                `json:"mods"`                // ordered list of mod IDs (earlier = lower priority)
	Overrides map[string]ModOverrides `json:"overrides,omitempty"` // per-mod destination overrides; key = modID

	// FomodSelections holds a per-profile FOMOD file selection that overrides
	// the mod's own default (meta.FomodFiles), keyed by modID. A mod absent
	// from this map deploys using its mod-level default.
	FomodSelections map[string][]FomodFileEntry `json:"fomod_selections,omitempty"`
}

// FomodEntriesFor returns this profile's FOMOD selection override for modID,
// or nil if the profile has none (meaning the mod's default applies).
func (p *Profile) FomodEntriesFor(modID string) []FomodFileEntry {
	if p.FomodSelections == nil {
		return nil
	}
	return p.FomodSelections[modID]
}

// SetFomodSelection sets (or, given an empty slice, clears) this profile's
// FOMOD selection override for modID.
func (p *Profile) SetFomodSelection(modID string, entries []FomodFileEntry) {
	if len(entries) == 0 {
		delete(p.FomodSelections, modID)
		return
	}
	if p.FomodSelections == nil {
		p.FomodSelections = map[string][]FomodFileEntry{}
	}
	p.FomodSelections[modID] = entries
}

// ProfilePath returns the path for a profile's JSON file within a game's profile dir.
func ProfilePath(gameID, name string) string {
	return filepath.Join(config.GameProfilesDir(gameID), name+".json")
}

// LoadProfile loads a profile by game and name, returning an empty profile if not found.
func LoadProfile(gameID, name string) (*Profile, error) {
	data, err := os.ReadFile(ProfilePath(gameID, name))
	if os.IsNotExist(err) {
		return &Profile{GameID: gameID, Name: name}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load profile %q: %w", name, err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse profile %q: %w", name, err)
	}
	p.GameID = gameID // ensure set even for profiles saved before this field existed
	return &p, nil
}

// Save persists a profile to disk.
func (p *Profile) Save() error {
	dir := config.GameProfilesDir(p.GameID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ProfilePath(p.GameID, p.Name), data, 0644)
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

// DeleteProfile removes a saved profile by game and name.
func DeleteProfile(gameID, name string) error {
	return os.Remove(ProfilePath(gameID, name))
}

// ListProfiles returns names of all saved profiles for the given game install.
func ListProfiles(gameID string) ([]string, error) {
	entries, err := os.ReadDir(config.GameProfilesDir(gameID))
	if os.IsNotExist(err) {
		return nil, nil
	}
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
