package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/client"
	"plumelog-cli/internal/risk"
)

const monitorIndex = "plumelog_monitor_message_key"

func (a *App) newAlarmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alarm",
		Short: "Warning rules and monitor messages",
		RunE:  helpOnly,
	}
	cmd.AddCommand(a.newAlarmList(), a.newAlarmSave(), a.newAlarmDelete(), a.newAlarmMessages())
	return cmd
}

func (a *App) newAlarmList() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "list",
		Long: "POST /getWarningRuleList. A null result means the server has no Redis client.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.invoke(cmd, risk.ReadOnly(), client.Request{Method: "POST", Path: "/getWarningRuleList"})
		},
	}
	mark(cmd, risk.Read, "List warning rules")
	return cmd
}

func (a *App) newAlarmSave() *cobra.Command {
	var (
		id, file                           string
		appName, env, category, className  string
		receiver, webhook                  string
		ruleTime, errorCount, status, hook int
	)
	cmd := &cobra.Command{
		Use: "save",
		Long: `POST /saveWarningRuleList?id= with a WarningRule JSON body.
Pass --file for a raw body, or the field flags. hookServe defaults to 1 (DingTalk); 2 is WeCom.
The id is the Redis hash field, not part of the body.`,
		Example: `  plumelog alarm save --id rule-1 --app demo --error-count 5 --rule-time 1 --webhook-url https://example.invalid/hook`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return a.fail(cmd, risk.Write, errors.New("--id is required"))
			}
			var body []byte
			if file != "" {
				if changed(cmd, "app") || changed(cmd, "env") || changed(cmd, "class-name") {
					return a.fail(cmd, risk.Write, errors.New("--file conflicts with field flags"))
				}
				b, err := os.ReadFile(file)
				if err != nil {
					return a.fail(cmd, risk.Write, err)
				}
				if !json.Valid(bytes.TrimSpace(b)) {
					return a.fail(cmd, risk.Write, errors.New("--file is not JSON"))
				}
				body = bytes.TrimSpace(b)
			} else {
				rule := map[string]any{}
				if appName != "" {
					rule["appName"] = appName
				}
				if env != "" {
					rule["env"] = env
				}
				if category != "" {
					rule["appCategory"] = category
				}
				if className != "" {
					rule["className"] = className
				}
				if receiver != "" {
					rule["receiver"] = receiver
				}
				if webhook != "" {
					rule["webhookUrl"] = webhook
				}
				if changed(cmd, "error-count") {
					rule["errorCount"] = errorCount
				}
				if changed(cmd, "rule-time") {
					rule["time"] = ruleTime
				}
				if changed(cmd, "status") {
					rule["status"] = status
				}
				if changed(cmd, "hook-serve") {
					rule["hookServe"] = hook
				} else {
					rule["hookServe"] = 1
				}
				var err error
				body, err = json.Marshal(rule)
				if err != nil {
					return a.fail(cmd, risk.Write, err)
				}
			}
			q := url.Values{}
			q.Set("id", id)
			return a.invoke(cmd, risk.Mutation(), client.Request{Method: "POST", Path: "/saveWarningRuleList", Query: q, Body: body})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "rule id (required)")
	cmd.Flags().StringVar(&file, "file", "", "raw WarningRule JSON file")
	cmd.Flags().StringVar(&appName, "app", "", "appName")
	cmd.Flags().StringVar(&env, "env", "", "env")
	cmd.Flags().StringVar(&category, "app-category", "", "appCategory")
	cmd.Flags().StringVar(&className, "class-name", "", "className")
	cmd.Flags().StringVar(&receiver, "receiver", "", "receiver")
	cmd.Flags().StringVar(&webhook, "webhook-url", "", "webhookUrl")
	cmd.Flags().IntVar(&errorCount, "error-count", 0, "errorCount")
	cmd.Flags().IntVar(&ruleTime, "rule-time", 0, "WarningRule.time window")
	cmd.Flags().IntVar(&status, "status", 0, "status")
	cmd.Flags().IntVar(&hook, "hook-serve", 1, "hookServe: 1 DingTalk, 2 WeCom")
	mark(cmd, risk.Write, "Create or replace a warning rule")
	return cmd
}

func (a *App) newAlarmDelete() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:     "delete",
		Long:    "POST /deleteWarningRule?id=. This removes the rule from Redis.",
		Example: `  plumelog alarm delete --dry-run --id rule-1` + "\n  plumelog alarm delete --yes --id rule-1",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return a.fail(cmd, risk.High, errors.New("--id is required"))
			}
			q := url.Values{}
			q.Set("id", id)
			return a.invoke(cmd, risk.HighRisk(false, false), client.Request{Method: "POST", Path: "/deleteWarningRule", Query: q})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "rule id (required)")
	mark(cmd, risk.High, "Delete a warning rule")
	return cmd
}

func (a *App) newAlarmMessages() *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use:     "messages",
		Long:    "POST /query?index=plumelog_monitor_message_key. Pass --dsl / --dsl-file or accept a match_all plus the default time window.",
		Example: `  plumelog alarm messages --size 20`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f.index = monitorIndex
			req, err := a.planSearch(cmd, f, false, 0)
			if err != nil {
				return a.fail(cmd, risk.Read, err)
			}
			return a.invoke(cmd, risk.ReadOnly(), req)
		},
	}
	cmd.Flags().StringVar(&f.dsl, "dsl", "", "raw JSON body")
	cmd.Flags().StringVar(&f.dslFile, "dsl-file", "", "raw JSON body file")
	cmd.Flags().StringVar(&f.fromTime, "from-time", "", "dtTime gte when the body is generated")
	cmd.Flags().StringVar(&f.toTime, "to-time", "", "dtTime lt when the body is generated")
	cmd.Flags().IntVar(&f.size, "size", 0, "page size")
	cmd.Flags().IntVar(&f.from, "from", 0, "page offset")
	mark(cmd, risk.Read, "Search alarm monitor messages")
	return cmd
}
