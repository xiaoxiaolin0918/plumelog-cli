package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/risk"
)

func (a *App) newTraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace",
		Short: "Look up a trace by traceId",
		RunE:  helpOnly,
	}
	cmd.AddCommand(a.newTraceGetCmd())
	return cmd
}

func (a *App) newTraceGetCmd() *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use: "get",
		Long: `POST /clientQuery?trace=true with a match_phrase on traceId.
Sort is dtTime asc, seq asc. The default page size is 1000.
Run-log index names look like plume_log_run_YYYYMMDD; trace indices look like plume_log_trace_YYYYMMDD. The server picks the dates from clientStartDate and clientEndDate.`,
		Example: `  plumelog trace get --trace-id TRACE_ID
  plumelog trace get --trace-id TRACE_ID --from-time 1700000000000 --to-time 1700003600000`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if f.traceID == "" {
				return a.fail(cmd, risk.Read, errors.New("--trace-id is required"))
			}
			req, err := a.planSearch(cmd, f, true, 1000)
			if err != nil {
				return a.fail(cmd, risk.Read, err)
			}
			return a.invoke(cmd, risk.ReadOnly(), req)
		},
	}
	cmd.Flags().StringVar(&f.traceID, "trace-id", "", "traceId to load (required)")
	cmd.Flags().StringVar(&f.app, "app", "", "optional appName match_phrase")
	cmd.Flags().StringVar(&f.fromTime, "from-time", "", "clientStartDate")
	cmd.Flags().StringVar(&f.toTime, "to-time", "", "clientEndDate")
	cmd.Flags().IntVar(&f.size, "size", 0, "page size (default 1000)")
	cmd.Flags().IntVar(&f.from, "from", 0, "page offset")
	cmd.Flags().BoolVar(&f.noHighlight, "no-highlight", false, "omit the content highlight block")
	mark(cmd, risk.Read, "Fetch one trace from the trace indices")
	return cmd
}
