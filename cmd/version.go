package cmd

import (
	"github.com/spf13/cobra"

	"plumelog-cli/internal/risk"
	"plumelog-cli/internal/version"
)

func (a *App) newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a.format = "json"
			return a.ok(cmd, map[string]string{
				"name":    version.Name,
				"version": version.Version,
			})
		},
	}
	mark(cmd, risk.Read, "Print the CLI version")
	return cmd
}
