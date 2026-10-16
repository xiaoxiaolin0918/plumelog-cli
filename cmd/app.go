package cmd

import (
	"errors"
	"net/url"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/client"
	"plumelog-cli/internal/risk"
)

func (a *App) newAppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Application names and extend fields",
		RunE:  helpOnly,
	}
	cmd.AddCommand(a.newAppsCmd("list"), a.newExtendCmd())
	return cmd
}

func (a *App) newExtendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extend",
		Short: "Per-app extend field map (MDC)",
		RunE:  helpOnly,
	}
	cmd.AddCommand(a.newExtendList(), a.newExtendAdd(), a.newExtendDel())
	return cmd
}

func (a *App) newExtendList() *cobra.Command {
	var appName string
	cmd := &cobra.Command{
		Use:     "list",
		Example: `  plumelog app extend list --app demo`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if appName == "" {
				return a.fail(cmd, risk.Read, errors.New("--app is required"))
			}
			q := url.Values{}
			q.Set("appName", appName)
			return a.invoke(cmd, risk.ReadOnly(), client.Request{Method: "POST", Path: "/getExtendfieldList", Query: q})
		},
	}
	cmd.Flags().StringVar(&appName, "app", "", "appName (required)")
	mark(cmd, risk.Read, "List extend fields for an app")
	return cmd
}

func (a *App) newExtendAdd() *cobra.Command {
	var appName, field, fieldName string
	cmd := &cobra.Command{
		Use:     "add",
		Example: `  plumelog app extend add --app demo --field userId --field-name "user id"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if appName == "" || field == "" || fieldName == "" {
				return a.fail(cmd, risk.Write, errors.New("--app, --field, and --field-name are required"))
			}
			q := url.Values{}
			q.Set("appName", appName)
			q.Set("field", field)
			q.Set("fieldName", fieldName)
			return a.invoke(cmd, risk.Mutation(), client.Request{Method: "POST", Path: "/addExtendfield", Query: q})
		},
	}
	cmd.Flags().StringVar(&appName, "app", "", "appName (required)")
	cmd.Flags().StringVar(&field, "field", "", "field key (required)")
	cmd.Flags().StringVar(&fieldName, "field-name", "", "display name (required)")
	mark(cmd, risk.Write, "Add an extend field")
	return cmd
}

func (a *App) newExtendDel() *cobra.Command {
	var appName, field string
	cmd := &cobra.Command{
		Use:  "del",
		Long: "POST /delExtendfield. Removes one extend-field mapping for an app.",
		Example: `  plumelog app extend del --dry-run --app demo --field userId
  plumelog app extend del --yes --app demo --field userId`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if appName == "" || field == "" {
				return a.fail(cmd, risk.High, errors.New("--app and --field are required"))
			}
			q := url.Values{}
			q.Set("appName", appName)
			q.Set("field", field)
			return a.invoke(cmd, risk.HighRisk(false, false), client.Request{Method: "POST", Path: "/delExtendfield", Query: q})
		},
	}
	cmd.Flags().StringVar(&appName, "app", "", "appName (required)")
	cmd.Flags().StringVar(&field, "field", "", "field key (required)")
	mark(cmd, risk.High, "Delete an extend field")
	return cmd
}
