# ITR Field Kit - Mod Manager

## Plan

### Architecture
- Language: Go (single binary, cross-platform, no runtime deps)
- CLI: cobra
- Data: JSON config files in XDG data dir (Linux) / AppData (Windows)

### Game structure (from vortex ext)
- `gameRoot/IntoTheRadius2/Content/Paks/Mods/`       PAK mods
- `gameRoot/IntoTheRadius2/Content/Paks/LogicMods/`  Logic blueprint mods
- `gameRoot/IntoTheRadius2/Content/Paks/LuaMods/`    Lua mods
- `gameRoot/IntoTheRadius2/Binaries/Win64/`           UE4SS DLL shim
- Custom files: relative to gameRoot

### Data layout (~/.local/share/itr-field-kit/ on Linux)
```
config.json
mods/{id}/meta.json
mods/{id}/files/          (extracted mod content)
mods/archives/{id}.zip    (original zips)
profiles/{name}.json
backups/{game-id}/{relative-path}   (backed-up game files, per install)
backups/{game-id}/registry.json     (tracks what's backed up)
state.json                          (current deployment state)
```

### CLI commands
- `itr game add <path> [--name <n>]`
- `itr game list`
- `itr game use <id>`
- `itr mod import <zip>`
- `itr mod list`
- `itr mod remove <id>`
- `itr profile new <name>`
- `itr profile list`
- `itr profile use <name>`
- `itr profile add <mod-id>`
- `itr profile remove <mod-id>`
- `itr deploy`
- `itr undeploy`
- `itr status`

### Mod type detection (from vortex testSupportedContent)
- UE4SS: has `ue4ss/UE4SS.dll`
- Lua mod: has `enabled.txt` (+ `main.lua`) or `shared/*.lua`
- PAK mod: has `.pak` files
- Custom: has `custom.txt`

### File mapping (port of vortex installContent)
- custom.txt: files go to game root relative paths (backup-eligible)
- UE4SS: dwmapi.dll → Binaries/Win64, rest → Content/Paks
- enabled.txt: entire mod dir → LuaMods/{modName}/
- shared/*.lua: → LuaMods/shared/{libName}/
- LogicMods/*.pak: → LogicMods/{modName}/
- *.pak: → Mods/{modName}/

### Backup logic (per game install, not per profile)
- Before symlinking file that replaces a REGULAR file: copy original to backup/registry
- On undeploy: remove symlink, restore backup
- On re-deploy: if target is regular file again (restored), update backup before symlinking

## Tasks

- [x] go.mod + main.go skeleton
- [x] internal/config - config types and persistence
- [x] internal/modmgr/types.go - mod + profile types
- [x] internal/modmgr/mapper.go - file mapping logic
- [x] internal/modmgr/import.go - zip import + extraction
- [x] internal/deploy/deploy.go - symlink + backup system
- [x] cmd/root.go - root command
- [x] cmd/game.go - game management
- [x] cmd/mod.go - mod management
- [x] cmd/profile.go - profile management
- [x] cmd/deploy.go + undeploy.go
- [x] go.sum (go mod tidy)
- [x] Verify build compiles

## Review

Build: clean
File mapping verified against vortex reference output – all 7 instruction types correct.
Smoke test: `mod import example-full.zip` correctly identifies types lua+shared+pak+custom,
maps all files to the right game-relative paths.
