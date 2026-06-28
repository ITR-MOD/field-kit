package fomod

import (
	"encoding/xml"
	"fmt"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Parse decodes a ModuleConfig.xml document.
//
// Real-world FOMODs come in whatever encoding their authoring tool happened
// to save in, regardless of what the XML prologue claims:
//   - Some are UTF-16 (LE or BE) with a BOM, despite declaring "utf-8".
//   - Some are Windows-1252 (curly quotes etc. in descriptions), also
//     despite declaring "utf-8".
//
// Detect a BOM first (covers UTF-16/UTF-8-with-BOM); if the result still
// isn't valid UTF-8, fall back to a Windows-1252 transcode rather than
// failing outright.
func Parse(data []byte) (*Config, error) {
	if decoded, _, err := transform.Bytes(unicode.BOMOverride(unicode.UTF8.NewDecoder()), data); err == nil {
		data = decoded
	}
	if !utf8.Valid(data) {
		if decoded, err := charmap.Windows1252.NewDecoder().Bytes(data); err == nil {
			data = decoded
		}
	}
	var cfg Config
	if err := xml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse ModuleConfig.xml: %w", err)
	}
	return &cfg, nil
}
