package config

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

// Game version identifiers.
const (
	VersionITR2     = "itr2"     // Into the Radius 2 (any version)
	VersionITR1_07a = "itr1-0.7a"
	VersionITR1_10  = "itr1-1.0"
	VersionITR1_27  = "itr1-2.7"
	VersionUnknown  = "unknown"
)

// itr1Hashes maps the MD5 of IntoTheRadius.exe to a version string.
var itr1Hashes = map[string]string{
	"6afcb6aad4058765984b09bc971391fe": VersionITR1_07a,
	"fa0617febdd034c4ee9a9177070bf9aa": VersionITR1_10,
	"8529de8a18f9eee0e2b66fc207b69cec": VersionITR1_27,
}

// IsITR1 returns true for any Into the Radius 1 version.
func IsITR1(version string) bool {
	return version == VersionITR1_07a || version == VersionITR1_10 || version == VersionITR1_27
}

// DetectGameVersion inspects the game directory and returns a version constant.
// Detection order:
//  1. If IntoTheRadius2.exe exists → itr2
//  2. If IntoTheRadius.exe exists, hash it and match against known ITR1 hashes
//  3. If IntoTheRadius.exe exists but hash unknown → itr1-unknown (treated as ITR1)
//  4. Neither found → unknown
func DetectGameVersion(gamePath string) string {
	if fileExists(filepath.Join(gamePath, "IntoTheRadius2.exe")) {
		return VersionITR2
	}
	exePath := filepath.Join(gamePath, "IntoTheRadius.exe")
	if !fileExists(exePath) {
		return VersionUnknown
	}
	hash, err := md5File(exePath)
	if err != nil {
		return VersionUnknown
	}
	if v, ok := itr1Hashes[hash]; ok {
		return v
	}
	// Exe found but hash not in our table — treat as ITR1 generic.
	return VersionITR1_27
}

// VersionLabel returns a human-readable label for a version string.
func VersionLabel(version string) string {
	switch version {
	case VersionITR2:
		return "ITR2"
	case VersionITR1_07a:
		return "ITR1 v0.7a"
	case VersionITR1_10:
		return "ITR1 v1.0"
	case VersionITR1_27:
		return "ITR1 v2.7"
	default:
		return "Unknown"
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func md5File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
