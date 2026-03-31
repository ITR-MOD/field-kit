// Package modmgr handles mod import, metadata, and file mapping.
//
// The mapper is a direct port of the installContent / testSupportedContent
// logic from the official Vortex extension (ITR_MOD_VORTEX_EXT/index.js).
// Destination paths are relative to the game root (the Steam app folder that
// contains IntoTheRadius2.exe at its root and the IntoTheRadius2/ project dir
// as a sub-folder).
package modmgr

import (
	"path/filepath"
	"strings"
)

// Game directory constants – relative to game root.
const (
	GameInternalID = "IntoTheRadius2"
	PakDir         = "IntoTheRadius2/Content/Paks"
	BinDir         = "IntoTheRadius2/Binaries/Win64"
)

// validExtensions mirrors the Vortex extension's VALID_EXTENSIONS list.
var validExtensions = map[string]bool{
	".pak": true, ".utoc": true, ".ucas": true,
	".lua": true, ".ini": true, ".txt": true, ".dll": true,
}

// Instruction describes a single file to deploy.
type Instruction struct {
	// Source is the relative path within the mod's files/ directory.
	// Empty when WriteContent is set.
	Source string
	// Dest is the relative path within the game root where the file should appear.
	Dest string
	// IsCustom is true for files that use the "custom" placement path,
	// meaning they may overwrite real game files and therefore require backup.
	IsCustom bool
	// WriteContent, when non-empty, means write this literal string to Dest
	// as a plain file instead of creating a symlink. Used for manager-provided
	// assets like override.txt whose content is fixed and not part of any mod zip.
	WriteContent string
}

// DetectTypes returns all mod types present in the given file list.
func DetectTypes(files []string) []ModType {
	hasUE4SS := anyMatch(files, func(f string) bool {
		return base(f) == "UE4SS.dll" && dir(f) == "ue4ss"
	})
	hasEnabledTxt := anyMatch(files, func(f string) bool { return base(f) == "enabled.txt" })
	hasMainLua := anyMatch(files, func(f string) bool { return base(f) == "main.lua" })
	hasShared := anyMatch(files, func(f string) bool { return base(f) == "shared" || dir(f) == "shared" || strings.Contains(f, "/shared/") })
	hasPak := anyMatch(files, func(f string) bool { return strings.ToLower(filepath.Ext(f)) == ".pak" })
	hasCustom := anyMatch(files, func(f string) bool { return base(f) == "custom.txt" })

	var types []ModType
	if hasUE4SS {
		types = append(types, ModTypeUE4SS)
	}
	if (hasEnabledTxt && hasMainLua) || (hasEnabledTxt) {
		types = append(types, ModTypeLua)
	}
	if hasShared {
		types = append(types, ModTypeShared)
	}
	if hasPak {
		types = append(types, ModTypePak)
	}
	if hasCustom {
		types = append(types, ModTypeCustom)
	}
	if len(types) == 0 {
		types = []ModType{ModTypeUnknown}
	}
	return types
}

// IsSupported returns true if the file list contains a recognised mod type.
func IsSupported(files []string) bool {
	for _, t := range DetectTypes(files) {
		if t != ModTypeUnknown {
			return true
		}
	}
	return false
}

// MapFiles maps mod files to their game-root-relative destinations.
// This is a port of installContent() from the Vortex extension.
func MapFiles(files []string) []Instruction {
	var instructions []Instruction
	alreadyCopied := map[string]bool{}

	// ── Custom format ──────────────────────────────────────────────────────
	// Files in the same directory as a custom.txt are placed at their path
	// relative to game root (stripping nothing – the zip itself encodes the
	// game-relative path, e.g. IntoTheRadius2/Content/ITR2/…).
	for _, f := range files {
		if base(f) != "custom.txt" {
			continue
		}
		customDir := dir(f)
		for _, sibling := range files {
			if dir(sibling) != customDir {
				continue
			}
			if base(sibling) == "custom.txt" {
				continue
			}
			if alreadyCopied[sibling] {
				continue
			}
			instructions = append(instructions, Instruction{
				Source:   sibling,
				Dest:     filepath.ToSlash(sibling),
				IsCustom: true,
			})
			alreadyCopied[sibling] = true
		}
	}

	// ── UE4SS ──────────────────────────────────────────────────────────────
	// Detect by presence of ue4ss/UE4SS.dll in the archive.
	if anyMatch(files, func(f string) bool {
		return base(f) == "UE4SS.dll" && dir(f) == "ue4ss"
	}) {
		findFirst := func(name, inDir string) string {
			for _, f := range files {
				if base(f) == name && (inDir == "" || dir(f) == inDir) {
					return f
				}
			}
			return ""
		}
		if v := findFirst("dwmapi.dll", ""); v != "" {
			instructions = append(instructions, Instruction{Source: v, Dest: BinDir + "/dwmapi.dll"})
		}
		// override.txt tells UE4SS (loaded by dwmapi.dll in Binaries/Win64) to
		// look for its DLL and config in Content/Paks instead of Binaries/Win64.
		// This is a fixed manager-provided asset — not from the mod zip.
		instructions = append(instructions, Instruction{
			Dest:         BinDir + "/override.txt",
			WriteContent: "../../Content/Paks",
		})
		if v := findFirst("UE4SS.dll", "ue4ss"); v != "" {
			instructions = append(instructions, Instruction{Source: v, Dest: PakDir + "/UE4SS.dll"})
		}
		if v := findFirst("UE4SS-settings.ini", "ue4ss"); v != "" {
			instructions = append(instructions, Instruction{Source: v, Dest: PakDir + "/UE4SS-settings.ini"})
		}
		// The Mods folder is mapped to LuaMods.
		for _, f := range files {
			if strings.HasPrefix(f, "ue4ss/Mods/") {
				rel := strings.TrimPrefix(f, "ue4ss/Mods/")
				instructions = append(instructions, Instruction{
					Source: f,
					Dest:   PakDir + "/LuaMods/" + rel,
				})
			}
		}
		return instructions
	}

	// ── Per-file routing ───────────────────────────────────────────────────
	luaSharedCopied := false

	for _, f := range files {
		if alreadyCopied[f] {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f))
		if !validExtensions[ext] {
			continue
		}

		fileDir := dir(f)
		baseDir := base(fileDir)

		// Determine Lua mod name (mirrors vortex logic).
		var luaModName string
		if strings.ToLower(baseDir) == "luamods" || strings.ToLower(baseDir) == "luamod" {
			luaModName = base(dir(fileDir))
		} else {
			luaModName = baseDir
		}

		// ── enabled.txt → Lua mod ──────────────────────────────────────────
		if base(f) == "enabled.txt" {
			luaModDir := fileDir
			instructions = append(instructions,
				Instruction{
					Source: luaModDir + "/enabled.txt",
					Dest:   PakDir + "/LuaMods/" + luaModName + "/enabled.txt",
				},
				Instruction{
					Source: luaModDir + "/Scripts",
					Dest:   PakDir + "/LuaMods/" + luaModName + "/Scripts",
				},
			)
			alreadyCopied[f] = true
			continue
		}

		// ── shared Lua library ─────────────────────────────────────────────
		if base(fileDir) == "shared" && ext == ".lua" && !luaSharedCopied {
			luaSharedCopied = true
			libName := luaModName
			if libName == "shared" {
				parent := base(dir(fileDir))
				if parent == "" || parent == "." {
					libName = "ITR2-Common"
				} else {
					libName = parent
				}
			}
			instructions = append(instructions, Instruction{
				Source: dir(f), // copy entire shared/ dir
				Dest:   PakDir + "/LuaMods/shared/" + libName,
			})
			alreadyCopied[f] = true
			continue
		}

		// ── .pak / .ucas / .utoc ──────────────────────────────────────────
		if ext == ".pak" || ext == ".ucas" || ext == ".utoc" {
			parentFolder := base(fileDir)
			var dest string
			if parentFolder == "LogicMods" {
				modName := base(dir(fileDir))
				dest = filepath.ToSlash(filepath.Join(PakDir, "LogicMods", modName, base(f)))
			} else {
				modName := base(fileDir)
				dest = filepath.ToSlash(filepath.Join(PakDir, "Mods", modName, base(f)))
			}
			instructions = append(instructions, Instruction{Source: f, Dest: dest})
			alreadyCopied[f] = true
		}
	}

	return instructions
}

// ── helpers ────────────────────────────────────────────────────────────────

func base(p string) string  { return filepath.Base(filepath.ToSlash(p)) }
func dir(p string) string   { return filepath.ToSlash(filepath.Dir(filepath.ToSlash(p))) }

func anyMatch(files []string, pred func(string) bool) bool {
	for _, f := range files {
		if pred(f) {
			return true
		}
	}
	return false
}
