# Plan: Manual File Adjustment (Destination Override)

## Context
Users need to manually rearrange where a mod's files get deployed — either globally for a mod, or per-profile for load order tuning. The mapper auto-routes files using Vortex logic, but edge cases (conflicts, custom load order) require the ability to override any file's `Dest` without touching the mod zip or the mapper.

Two scopes:
- **Mod-level override**: always applied when the mod deploys (any profile)
- **Profile-level override**: only applied when that specific profile is active; takes precedence over mod-level

---

## Data Layer

### 1. `internal/modmgr/overrides.go` *(new file)*

```go
// ModOverrides holds per-source-file destination overrides for a mod.
type ModOverrides struct {
    // Sources maps a mod source path → override game-root-relative dest.
    Sources map[string]string `json:"sources,omitempty"`
}

func OverridesPath(modID string) string  // {dataDir}/mods/{modID}/overrides.fk.json
func LoadModOverrides(modID string) (*ModOverrides, error)
func SaveModOverrides(modID string, o *ModOverrides) error
```

### 2. Extend `Profile` in `internal/modmgr/types.go`

Profile embeds per-mod overrides directly (same `ModOverrides` structure, keyed by modID):

```go
type Profile struct {
    Name      string                 `json:"name"`
    Mods      []string               `json:"mods"`
    Overrides map[string]ModOverrides `json:"overrides,omitempty"`
    // key: modID  value: same structure as mods/{id}/overrides.fk.json
}
```

`Profile.Save()` / `LoadProfile()` already handle serialisation — no changes needed there.

### 3. `internal/deploy/deploy.go` — inject overrides after mapping

Priority: **Profile > Mod > Default**. Profile overrides are checked first; if a source has a profile-level override, the mod-level override for that source is skipped entirely.

```go
modOvr, _ := modmgr.LoadModOverrides(modID)
for i, instr := range instructions {
    src := instr.Source
    // 1. Profile-level override — highest priority, short-circuits mod-level
    if profile.Overrides != nil {
        if profMod, ok := profile.Overrides[modID]; ok {
            if dest, ok := profMod.Sources[src]; ok {
                instructions[i].Dest = dest
                continue
            }
        }
    }
    // 2. Mod-level override — only applied if profile didn't override this source
    if modOvr != nil {
        if dest, ok := modOvr.Sources[src]; ok {
            instructions[i].Dest = dest
        }
    }
    // 3. Default mapper output — unchanged
}
```

---

## TUI Layer

### 4. New overlay kind in `tui/model.go`

```go
overlayAdjust // editable instruction list for mod-level or profile-level overrides
```

### 5. New model state fields

```go
adjustInstructions []modmgr.Instruction // current instructions (post-mapper)
adjustOverrides    map[string]string     // active overrides being edited (source → dest)
adjustModID        string                // mod being edited
adjustCursor       int                   // row cursor in overlay
adjustScroll       int                   // scroll offset
adjustIsProfile    bool                  // false = mod-level, true = profile-level
```

### 6. Trigger keys

| Tab / Pane | Key | Action |
|---|---|---|
| Mods tab | `m` | Open adjust overlay for selected mod (mod-level) |
| Profiles right pane (mod list) | `m` | Open adjust overlay for that mod scoped to the active profile |

### 7. Overlay behaviour (`updateAdjustOverlay`)

- `↑/↓` or `j/k` — move cursor
- `Enter` — open `overlayInput` pre-filled with current `Dest` (or override) to edit destination
  - On confirm: write into `adjustOverrides[source] = newDest`
- `r` — reset row to mapper default (delete key from `adjustOverrides`)
- `Esc` / `s` — save and close:
  - Mod-level: `modmgr.SaveModOverrides(adjustModID, &ModOverrides{Sources: adjustOverrides})`
  - Profile-level: write into `profile.Overrides[adjustModID]`, call `profile.Save()`

### 8. Overlay rendering (`renderAdjustOverlay` in `tui/view.go`)

- Title: `" Adjust: {modName}"` with scope suffix `[mod-level]` or `[profile: name]`
- Each row: `source → dest` — if dest differs from mapper default, render in yellow with `*` marker
- Rows with `WriteContent` are shown as `(manager-written)` and are non-editable (skip with cursor)
- Hint bar: `↑↓ nav   Enter edit dest   r reset   Esc/s save`

### 9. Update help bars (`tui/view.go` renderHelp)

- Mods tab: add `key("m", "adjust")`
- Profiles right pane: add `key("m", "adjust")`

---

## Files to modify

| File | Change |
|---|---|
| `internal/modmgr/overrides.go` | **New** — ModOverrides type + Load/Save/Path |
| `internal/modmgr/types.go` | Add `Overrides` field to Profile |
| `internal/deploy/deploy.go` | Apply override layers after MapFilesForGame |
| `tui/model.go` | New overlay kind + state fields + key handlers + open helpers + save logic |
| `tui/view.go` | `renderAdjustOverlay()` + help bar updates |

---

## Verification

1. `go build ./...` — clean build
2. Import a mod, press `m` on it in Mods tab → overlay opens showing mapper-generated instructions
3. Edit a dest for one file → row turns yellow with `*` marker
4. Press `Esc` → deploy the profile → confirm symlink lands at the overridden dest, not the mapper dest
5. Open profile right pane, press `m` on a mod → same overlay, but changes are saved to `profile.Overrides`
6. Deploy with both mod-level and profile-level override on the same file → profile-level wins
7. Press `r` on an overridden row → resets to default; redeploy confirms original dest is used
8. Inspect `mods/{id}/overrides.json` and `profiles/{name}.json` on disk to confirm JSON is correct
