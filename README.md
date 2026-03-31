# ITR Field Kit

A symlink-based mod manager for **Into the Radius 2**, built for Linux (with Windows support).

Mods are stored in a local cache and deployed to the game directory via symlinks — no copying, instant enable/disable, and original game files are automatically backed up and restored.

---

## Features

- **Non-destructive installs** — symlinks mean zero file duplication; undeploying is instant
- **Automatic backups** — any game file a mod would overwrite is backed up per-installation before the symlink is created, and restored on undeploy
- **Profile system** — multiple mod profiles; switch between them without re-importing anything
- **All mod types supported** — PAK, LogicMods, Lua mods, shared Lua libraries, UE4SS, and custom file placements
- **TUI** — monochrome green terminal UI, keyboard-driven, works over SSH
- **CLI** — every action is also available as a direct subcommand for scripting

---

## Installation

### From source

Requires **Go 1.22+**.

```sh
git clone https://github.com/Merith-TK/itr-field-kit
cd itr-field-kit
go build -o itr .
```

Move the resulting `itr` binary somewhere on your `$PATH`, e.g. `~/.local/bin/`.

### On Windows

Build the same way. Symlink creation requires either **Developer Mode** (Settings → System → Developer Mode) or running as Administrator.

---

## Quick Start

```sh
# 1. Register your game installation
itr game add "/path/to/steamapps/common/IntoTheRadius2"

# 2. Import a mod zip
itr mod import ~/Downloads/MyCoolMod_v1.0.zip

# 3. Add the mod to your active profile
itr profile add MyCoolMod_v1.0

# 4. Deploy
itr deploy
```

Or just run `itr` to use the interactive TUI.

---

## TUI

Run `itr` with no arguments to open the terminal UI.

```
  ITR FIELD KIT   [1]GAMES  [2]MODS  [3]PROFILES  [4]DEPLOY
 ─────────────────────────────────────────────────────────────
  (content area)
 ─────────────────────────────────────────────────────────────
  ✓ Ready
  1-4:tabs  Tab:next  ↑↓:nav  ...tab-specific keys...  q:quit
```

### Navigation

| Key | Action |
|-----|--------|
| `1` `2` `3` `4` | Switch between tabs |
| `Tab` / `Shift+Tab` | Cycle tabs |
| `↑` / `↓` (or `k`/`j`) | Move selection |
| `q` / `Ctrl+C` | Quit |

### GAMES tab `[1]`

Manage game installations. You need at least one registered game before deploying.

| Key | Action |
|-----|--------|
| `a` | Add a game — prompts for the installation path, then an optional display name |
| `Enter` / `Space` | Set the highlighted game as active |
| `d` | Remove the game from the manager (does not delete game files) |

> The active game is marked with `▶`. Game path = the Steam app folder that contains `IntoTheRadius2.exe` at its root.

### MODS tab `[2]`

Import and inspect mods.

| Key | Action |
|-----|--------|
| `i` | Import a mod — prompts for a `.zip` path (drag & drop onto the terminal works on most systems) |
| `Enter` | Show mod details and its install map (where each file will be symlinked) |
| `d` | Delete the mod from cache |

Mod types are detected automatically:

| Type | Detection | Destination |
|------|-----------|-------------|
| `pak` | `.pak` / `.ucas` / `.utoc` files | `IntoTheRadius2/Content/Paks/Mods/{name}/` |
| `logicmod` | `.pak` inside a `LogicMods/` folder | `IntoTheRadius2/Content/Paks/LogicMods/{name}/` |
| `lua` | `enabled.txt` present | `IntoTheRadius2/Content/Paks/LuaMods/{name}/` |
| `shared` | `.lua` files inside `shared/` | `IntoTheRadius2/Content/Paks/LuaMods/shared/{name}/` |
| `ue4ss` | `ue4ss/UE4SS.dll` present | Paks dir + Binaries/Win64 |
| `custom` | `custom.txt` present | Game root (relative path, **backed up**) |

### PROFILES tab `[3]`

Profiles let you maintain multiple mod lists — e.g. a "vanilla+" profile and a "full overhaul" profile.

**Left pane — profiles list:**

| Key | Action |
|-----|--------|
| `n` | Create a new profile |
| `Enter` / `Space` | Set highlighted profile as active (used on next deploy) |
| `→` / `Tab` | Move focus to the mod list on the right |
| `d` | Delete the profile |

**Right pane — mod toggle list** (press `→` or `Tab` to enter):

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate mod list |
| `Space` / `Enter` | Toggle mod on/off in the selected profile |
| `←` / `h` | Return to profiles list |

Mods marked `[✓]` are enabled in the selected profile. Order in the list = deployment order (lower = deployed first, higher = higher priority).

### DEPLOY tab `[4]`

Shows current game, profile, and deployment status.

| Key | Action |
|-----|--------|
| `d` | Deploy active profile to active game (confirms before proceeding) |
| `u` | Undeploy — removes all symlinks and restores original game files |
| `r` | Refresh deployment status display |

---

## CLI Reference

All operations available in the TUI are also available as subcommands.

### Games

```sh
itr game add <path> [--name "Display Name"]   # register a game installation
itr game list                                  # list registered games
itr game use <id>                              # set active game
```

### Mods

```sh
itr mod import <path/to/mod.zip>   # import and cache a mod
itr mod list                       # list all imported mods
itr mod info <mod-id>              # show details + install map
itr mod remove <mod-id>            # remove mod from cache
```

### Profiles

```sh
itr profile new <name>           # create a profile
itr profile list                 # list all profiles
itr profile use <name>           # switch active profile
itr profile add <mod-id>         # add mod to active profile
itr profile remove <mod-id>      # remove mod from active profile
itr profile show [name]          # list mods in a profile
```

### Deploy

```sh
itr deploy      # deploy active profile → active game
itr undeploy    # remove symlinks + restore backups
itr status      # show current deployment state
```

---

## Data Storage

All data is stored in the XDG data directory (Linux) or AppData (Windows):

| Platform | Path |
|----------|------|
| Linux | `~/.local/share/itr-field-kit/` |
| Windows | `%APPDATA%\itr-field-kit\` |

```
itr-field-kit/
├── config.json              ← registered games, active game/profile
├── mods/
│   ├── {mod-id}/
│   │   ├── meta.json        ← mod metadata + detected types
│   │   └── files/           ← extracted mod contents
│   └── archives/
│       └── {mod-id}.zip     ← original zip (kept for reference)
├── profiles/
│   └── {name}.json          ← ordered list of mod IDs
├── backups/
│   └── {game-id}/           ← one folder per game installation
│       └── ...              ← backed-up original game files
└── state.json               ← current deployment record
```

---

## Backup & Restore

When a mod uses the **custom** placement format (placing files at arbitrary paths relative to the game root — common for config/INI tweaks), the manager:

1. **Before deploying:** copies the current game file to `backups/{game-id}/` (updating the backup if one already exists — handles game patches between deploys)
2. **Replaces** the game file with a symlink to the mod's cached file
3. **On undeploy:** removes the symlink and copies the backup back

Backups are **per game installation** (not per profile), so switching profiles doesn't affect them. If the game updates a file between deploys, the backup is refreshed before the new symlink is created.

---

## Mod Packaging Guide (for mod authors)

See [`.references/ITR_MOD_PKG_EXAMPLE/readme.md`](.references/ITR_MOD_PKG_EXAMPLE/readme.md) for the full packaging reference. The short version:

```
mod.zip/
├── MyMod/
│   ├── MyMod.pak                     ← PAK mod
│   ├── enabled.txt                   ← marks as Lua mod
│   ├── Scripts/
│   │   └── main.lua
│   ├── shared/
│   │   └── mylib.lua                 ← shared Lua library
│   └── LogicMods/
│       └── MyLogic.pak               ← LogicMod
└── IntoTheRadius2/Content/ITR2/      ← custom placement
    └── IniSettings/
        ├── custom.txt                ← triggers custom install
        └── Settings.ini
```

---

## Linux / Proton Notes

- The game path should be the Steam common folder: `~/.steam/steam/steamapps/common/IntoTheRadius2/`
- Symlinks work natively; no special Proton configuration needed
- If using a non-default Steam library, use `itr game add <your-library-path>/common/IntoTheRadius2`

---

## License

See [LICENSE](LICENSE) if present, otherwise assume all rights reserved until a license is added.
