package cmd

import (
	"fmt"
	"os"

	"github.com/ITR-MOD/field-kit/internal/config"
	"github.com/spf13/cobra"
)

var gameCmd = &cobra.Command{
	Use:   "game",
	Short: "Manage game installations",
}

var gameAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Register a game installation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		gamePath := args[0]

		// Validate the path exists.
		if _, err := os.Stat(gamePath); err != nil {
			return fmt.Errorf("path does not exist: %s", gamePath)
		}

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			name = "Into the Radius 2"
		}

		g, err := config.AddGame(name, gamePath)
		if err != nil {
			return err
		}
		fmt.Printf("registered game %q (id: %s)\n", g.Name, g.ID)
		if config.Get().ActiveGame == g.ID {
			fmt.Println("set as active game")
		}
		return nil
	},
}

var gameListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered game installations",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Get()
		if len(cfg.Games) == 0 {
			fmt.Println("no games registered")
			return nil
		}
		for _, g := range cfg.Games {
			active := ""
			if g.ID == cfg.ActiveGame {
				active = " [active]"
			}
			fmt.Printf("  %s  %s  %s%s\n", g.ID, g.Name, g.Path, active)
		}
		return nil
	},
}

var gameUseCmd = &cobra.Command{
	Use:   "use <id>",
	Short: "Set the active game installation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.SetActiveGame(args[0]); err != nil {
			return err
		}
		fmt.Printf("active game set to %s\n", args[0])
		return nil
	},
}

func init() {
	gameAddCmd.Flags().String("name", "", "friendly name for this installation")
	gameCmd.AddCommand(gameAddCmd, gameListCmd, gameUseCmd)
}
