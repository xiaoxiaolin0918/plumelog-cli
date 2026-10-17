package cmd

import (
	"errors"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/xiaoxiaolin0918/plumelog-cli/internal/client"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/risk"
)

func (a *App) newAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Queue, index, and run-model operations",
		Long:  "High-risk deletes require --yes. queue clear and index delete also require --admin-password, sent as the query parameter adminPassWord. The value is never printed.",
		RunE:  helpOnly,
	}
	cmd.AddCommand(a.newAdminQueue(), a.newAdminIndex(), a.newAdminRunModel(), a.newAdminDeleteByQuery())
	return cmd
}

func (a *App) newAdminQueue() *cobra.Command {
	cmd := &cobra.Command{Use: "queue", Short: "Redis log queue", RunE: helpOnly}
	cmd.AddCommand(a.newQueueCounts(), a.newQueueClear())
	return cmd
}

func (a *App) newQueueCounts() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "counts",
		Long: "POST /getQueueCounts. Result fields are runSize and traceSize.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.invoke(cmd, risk.ReadOnly(), client.Request{Method: "POST", Path: "/getQueueCounts"})
		},
	}
	mark(cmd, risk.Read, "Show Redis queue depths")
	return cmd
}

func (a *App) newQueueClear() *cobra.Command {
	cmd := &cobra.Command{
		Use: "clear",
		Long: `POST /deleteQueue?adminPassWord=.
Deletes the run-log and trace-log Redis queues. Requires --yes and --admin-password.
--dry-run prints the URL with adminPassWord redacted and does not send it.`,
		Example: `  plumelog admin queue clear --dry-run
  plumelog admin queue clear --yes --admin-password "$PLUMELOG_ADMIN_PASSWORD"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.invoke(cmd, risk.HighRisk(true, true), client.Request{Method: "POST", Path: "/deleteQueue"})
		},
	}
	mark(cmd, risk.High, "Clear the Redis log queues")
	return cmd
}

func (a *App) newAdminIndex() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Elasticsearch index info, delete, and reset",
		Long:  "Run indices look like plume_log_run_YYYYMMDD. Trace indices look like plume_log_trace_YYYYMMDD.",
		RunE:  helpOnly,
	}
	cmd.AddCommand(a.newIndexInfo(), a.newIndexDelete(), a.newIndexReset())
	return cmd
}

func (a *App) newIndexInfo() *cobra.Command {
	var index string
	cmd := &cobra.Command{
		Use:     "info",
		Example: `  plumelog admin index info --index plume_log_run_*`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if index == "" {
				return a.fail(cmd, risk.Read, errors.New("--index is required"))
			}
			q := url.Values{}
			q.Set("index", index)
			return a.invoke(cmd, risk.ReadOnly(), client.Request{Method: "POST", Path: "/getServerInfo", Query: q})
		},
	}
	cmd.Flags().StringVar(&index, "index", "", "index or pattern passed to getServerInfo (required)")
	mark(cmd, risk.Read, "Show server index info")
	return cmd
}

func (a *App) newIndexDelete() *cobra.Command {
	var index string
	cmd := &cobra.Command{
		Use: "delete",
		Long: `POST /deleteIndex?index=&adminPassWord=.
Upstream deletes the index and recreates a run or trace index when the name matches those prefixes.
Requires --yes and --admin-password.`,
		Example: `  plumelog admin index delete --dry-run --index plume_log_run_20240101
  plumelog admin index delete --yes --index plume_log_run_20240101 --admin-password "$PLUMELOG_ADMIN_PASSWORD"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if index == "" {
				return a.fail(cmd, risk.High, errors.New("--index is required"))
			}
			q := url.Values{}
			q.Set("index", index)
			return a.invoke(cmd, risk.HighRisk(true, true), client.Request{Method: "POST", Path: "/deleteIndex", Query: q})
		},
	}
	cmd.Flags().StringVar(&index, "index", "", "index name (required)")
	mark(cmd, risk.High, "Delete an index")
	return cmd
}

func (a *App) newIndexReset() *cobra.Command {
	var index string
	cmd := &cobra.Command{
		Use: "reset",
		Long: `POST /resetIndex?index=.
Upstream does not check adminPassWord. This command still requires --yes.
--admin-password is not sent.`,
		Example: `  plumelog admin index reset --dry-run --index plume_log_run_20240101
  plumelog admin index reset --yes --index plume_log_run_20240101`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if index == "" {
				return a.fail(cmd, risk.High, errors.New("--index is required"))
			}
			q := url.Values{}
			q.Set("index", index)
			return a.invoke(cmd, risk.HighRisk(false, false), client.Request{Method: "POST", Path: "/resetIndex", Query: q})
		},
	}
	cmd.Flags().StringVar(&index, "index", "", "index name (required)")
	mark(cmd, risk.High, "Reset an index mapping")
	return cmd
}

func (a *App) newAdminRunModel() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "run-model",
		Long: "POST /getRunModel. Typical values are redis, kafka, rest, restServer, ui, or lite.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.invoke(cmd, risk.ReadOnly(), client.Request{Method: "POST", Path: "/getRunModel"})
		},
	}
	mark(cmd, risk.Read, "Show the server run model")
	return cmd
}

func (a *App) newAdminDeleteByQuery() *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use: "delete-by-query",
		Long: `POST /deleteByQuery?index=&size=&from= with an Elasticsearch DSL body.
Upstream does not check adminPassWord. This command requires --yes and a JSON body via --dsl or --dsl-file.
--admin-password is not placed on the wire.`,
		Example: `  plumelog admin delete-by-query --dry-run --index plume_log_run_20240101 --dsl '{"query":{"match_all":{}}}'
  plumelog admin delete-by-query --yes --index plume_log_run_20240101 --dsl-file delete.json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if f.index == "" {
				return a.fail(cmd, risk.High, errors.New("--index is required"))
			}
			if f.dsl == "" && f.dslFile == "" {
				return a.fail(cmd, risk.High, errors.New("--dsl or --dsl-file is required"))
			}
			req, err := a.planSearch(cmd, f, false, 0)
			if err != nil {
				return a.fail(cmd, risk.High, err)
			}
			req.Path = "/deleteByQuery"
			return a.invoke(cmd, risk.HighRisk(false, false), req)
		},
	}
	cmd.Flags().StringVar(&f.index, "index", "", "index list (required)")
	cmd.Flags().StringVar(&f.dsl, "dsl", "", "JSON body")
	cmd.Flags().StringVar(&f.dslFile, "dsl-file", "", "JSON body file")
	cmd.Flags().IntVar(&f.size, "size", 0, "size query parameter")
	cmd.Flags().IntVar(&f.from, "from", 0, "from query parameter")
	mark(cmd, risk.High, "Delete documents by Elasticsearch query")
	return cmd
}
