package cmd

import (
	"github.com/spf13/cobra"

	"plumelog-cli/internal/risk"
)

func (a *App) newWhoamiCmd() *cobra.Command {
	var probe bool
	cmd := &cobra.Command{
		Use: "whoami",
		Long: `Thin alias of auth status (same output path and fields).

Always prints (never secrets):
  base_url, authenticated, has_cookie, cookie_path, path_prefix

Cookie values and passwords are never printed. With --probe, POSTs /getRunModel
to refine authenticated from the server response.`,
		Example: `  plumelog whoami
  plumelog whoami --probe`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runAuthStatus(cmd, probe)
		},
	}
	cmd.Flags().BoolVar(&probe, "probe", false, "POST /getRunModel to see if the session is accepted")
	mark(cmd, risk.Read, "Show session identity metadata without secrets")
	return cmd
}
