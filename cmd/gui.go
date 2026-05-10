//go:build desktop

package cmd

import (
	"io/fs"
	"os"

	"github.com/ITR-MOD/field-kit/gui"
	"github.com/spf13/cobra"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

const isDesktopBuild = true

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch the graphical UI (Win98/NT style)",
	RunE:  runGUI,
}

func runGUI(_ *cobra.Command, _ []string) error {
	// Sub-path the embedded FS so Wails sees index.html at the root.
	frontendFS, err := fs.Sub(gui.FrontendFS, "frontend")
	if err != nil {
		return err
	}

	// Enable the WebKit inspector when --field-kit-debug is passed.
	openInspector := false
	if os.Getenv("FIELD_KIT_DEBUG") == "true" {
		openInspector = true
	}

	app := gui.NewApp()
	return wails.Run(&options.App{
		Title:     "ITR Field Kit v0.4.0",
		Width:     1200,
		Height:    768,
		MaxWidth:  1920,
		MaxHeight: 1080,
		MinWidth:  990,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: frontendFS,
		},
		OnStartup: app.Startup,
		Bind:      []interface{}{app},
		Debug: options.Debug{
			OpenInspectorOnStartup: openInspector,
		},
	})
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
