package cmd

import (
	"io/fs"

	"github.com/ITR-MOD/field-kit/gui"
	"github.com/spf13/cobra"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

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

	app := gui.NewApp()
	return wails.Run(&options.App{
		Title:  "ITR Field Kit v0.4.0",
		Width:  1200,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: frontendFS,
		},
		OnStartup: app.Startup,
		Bind:      []interface{}{app},
	})
}

func init() {
	rootCmd.AddCommand(guiCmd)
}

