package cmd

import (
	"fmt"

	"github.com/Merith-TK/itr-field-kit/internal/config"
	"github.com/Merith-TK/itr-field-kit/internal/deploy"
	"github.com/Merith-TK/itr-field-kit/internal/modmgr"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the active profile to the active game installation",
	RunE: func(cmd *cobra.Command, args []string) error {
		game := requireActiveGame()
		profileName := config.Get().ActiveProfile

		profile, err := modmgr.LoadProfile(profileName)
		if err != nil {
			return fmt.Errorf("load profile %q: %w", profileName, err)
		}

		if len(profile.Mods) == 0 {
			fmt.Printf("profile %q has no mods – nothing to deploy\n", profileName)
			return nil
		}

		fmt.Printf("deploying profile %q (%d mods) to %s …\n",
			profileName, len(profile.Mods), game.Path)

		if err := deploy.Deploy(game, profile); err != nil {
			return fmt.Errorf("deploy failed: %w", err)
		}

		fmt.Println("deploy complete")
		return nil
	},
}

var undeployCmd = &cobra.Command{
	Use:   "undeploy",
	Short: "Remove all deployed symlinks and restore original game files",
	RunE: func(cmd *cobra.Command, args []string) error {
		game := requireActiveGame()

		fmt.Printf("undeploying from %s …\n", game.Path)
		if err := deploy.Undeploy(game); err != nil {
			return fmt.Errorf("undeploy failed: %w", err)
		}
		fmt.Println("undeploy complete – original files restored")
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current deployment status",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := deploy.Status()
		if err != nil {
			return err
		}

		if len(state.Deployed) == 0 {
			fmt.Println("nothing deployed")
			return nil
		}

		fmt.Printf("deployed profile %q to game %s\n", state.Profile, state.GameID)
		fmt.Printf("%d symlinks active:\n", len(state.Deployed))
		for _, df := range state.Deployed {
			custom := ""
			if df.IsCustom {
				custom = " [custom]"
			}
			fmt.Printf("  [%s] %s%s\n", df.ModID, df.GamePath, custom)
		}
		return nil
	},
}
