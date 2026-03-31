package modmgr

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ITR-MOD/field-kit/internal/config"
)

// ModOverrides holds per-source-file destination overrides for a mod.
// If a source path has an entry here it replaces the mapper-generated Dest
// during deployment.
type ModOverrides struct {
	// Sources maps a mod-relative source path → game-root-relative dest override.
	Sources map[string]string `json:"sources,omitempty"`
}

// OverridesPath returns the on-disk path for a mod's override file.
func OverridesPath(modID string) string {
	return filepath.Join(config.ModsDir(), modID, "overrides.fk.json")
}

// LoadModOverrides loads overrides for a mod. Returns an empty struct (not nil)
// if no override file exists yet.
func LoadModOverrides(modID string) (*ModOverrides, error) {
	data, err := os.ReadFile(OverridesPath(modID))
	if os.IsNotExist(err) {
		return &ModOverrides{}, nil
	}
	if err != nil {
		return nil, err
	}
	var o ModOverrides
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

// SaveModOverrides persists overrides for a mod. If Sources is empty the file
// is removed (no point keeping an empty override file).
func SaveModOverrides(modID string, o *ModOverrides) error {
	path := OverridesPath(modID)
	if len(o.Sources) == 0 {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
