package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/client"
	"plumelog-cli/internal/dsl"
	"plumelog-cli/internal/risk"
)

type searchFlags struct {
	app, appName       string
	excludeApp         bool
	env                string
	level, logLevel    string
	traceID            string
	class, className   string
	server, serverName string
	content            string
	contentWildcard         bool
	noContentAutoWildcard  bool
	extend             []string
	fromTime, toTime   string
	size               int
	from, offset       int
	sort               string
	noHighlight        bool
	dsl, dslFile       string
	index, rng         string
	trace              bool
}

func (a *App) newQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Search logs, pass a raw DSL, or list app names",
		RunE:  helpOnly,
	}
	cmd.AddCommand(a.newSearchCmd(), a.newDSLCmd(), a.newAppsCmd("apps"))
	return cmd
}

func (a *App) newSearchCmd() *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use: "search",
		Long: `Build an Elasticsearch bool query the way Home.vue does and POST /clientQuery.

Shortcut filters become match_phrase clauses (appName, env, logLevel, traceId,
className, serverName), a query_string on content, match_phrase extend fields,
and a dtTime range. --exclude-app moves appName into must_not.
A dotted -q identifier (e.g. java.lang.OutOfMemoryError) is auto-wrapped as *...* so
ES wildcards can match the analyzed token; use --content-wildcard to force *value*
for bare fragments like OutOfMemoryError. Structured query_string (AND/OR, field:value,
quotes) is never wrapped, even with --content-wildcard. Use --no-content-auto-wildcard
to keep dotted identifiers literal.
Default sort is dtTime desc. A traceId switches sort to dtTime asc, seq asc.
Highlight is on unless --no-highlight.

--index or --range switches the endpoint to POST /query.
When --index is set and none of --from-time/--to-time/--range are given, the CLI
auto-aligns the dtTime window to the index YYYYMMDD (full calendar day) so
historical indexes do not silently return 0 hits against "today".
--range (day|week|month) is a server-relative window anchored at now; combining
it with a historical --index can still miss. Prefer --from-time/--to-time for
historical indexes. Explicit --from-time/--to-time always win for the body window.
--dsl / --dsl-file replaces the generated body and conflicts with shortcut flags.
On /clientQuery, clientStartDate/clientEndDate select daily indices; they default
to the configured window (1h) when omitted. A future --to-time is clamped to now.`,
		Example: `  plumelog query search --app demo --level ERROR -q timeout
  plumelog query search --trace-id abc --from-time 1700000000000 --to-time 1700003600000
  plumelog query search --dsl-file query.json --from-time 1700000000000 --to-time 1700003600000
  plumelog query search --index plume_log_run_20240101 --range day --app demo`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req, err := a.planSearch(cmd, f, false, 0)
			if err != nil {
				return a.fail(cmd, risk.Read, err)
			}
			return a.invoke(cmd, risk.ReadOnly(), req)
		},
	}
	bindSearchFlags(cmd, &f)
	mark(cmd, risk.Read, "Search logs via shortcut filters or a raw DSL")
	return cmd
}

func (a *App) newDSLCmd() *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use:  "dsl",
		Long: "POST a raw Elasticsearch JSON body. Read it from --file or stdin.\nDefault endpoint is /clientQuery. --index or --range selects /query. --trace adds trace=true.",
		Example: `  plumelog query dsl --file query.json
  cat query.json | plumelog query dsl --from-time 1700000000000 --to-time 1700003600000`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			body, err := readDSLInput(cmd, f.dslFile, f.dsl, true)
			if err != nil {
				return a.fail(cmd, risk.Read, err)
			}
			f.dsl = string(body)
			f.dslFile = ""
			req, err := a.planSearch(cmd, f, false, 0)
			if err != nil {
				return a.fail(cmd, risk.Read, err)
			}
			return a.invoke(cmd, risk.ReadOnly(), req)
		},
	}
	cmd.Flags().StringVar(&f.dslFile, "file", "", "JSON body file")
	cmd.Flags().StringVar(&f.dsl, "dsl", "", "raw JSON body")
	cmd.Flags().StringVar(&f.fromTime, "from-time", "", "clientStartDate (ms, RFC3339, or YYYY-MM-DD HH:MM:SS)")
	cmd.Flags().StringVar(&f.toTime, "to-time", "", "clientEndDate; a future value is clamped to now")
	cmd.Flags().IntVar(&f.size, "size", 0, "page size (default 100)")
	cmd.Flags().IntVar(&f.from, "from", 0, "page offset")
	cmd.Flags().StringVar(&f.index, "index", "", "switch to POST /query with this index")
	cmd.Flags().StringVar(&f.rng, "range", "", "POST /query range: day, week, or month")
	cmd.Flags().BoolVar(&f.trace, "trace", false, "search trace indices (clientQuery trace=true)")
	mark(cmd, risk.Read, "POST a raw Elasticsearch DSL")
	return cmd
}

func (a *App) newAppsCmd(use string) *cobra.Command {
	var source string
	cmd := &cobra.Command{
		Use:  use,
		Long: "cache (default) calls POST /getAppNames. es calls POST /queryAppNames with the Home.vue terms aggregation on appNameWithEnv.",
		Example: `  plumelog query apps
  plumelog query apps --source es`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req, err := appsRequest(source)
			if err != nil {
				return a.fail(cmd, risk.Read, err)
			}
			return a.invoke(cmd, risk.ReadOnly(), req)
		},
	}
	cmd.Flags().StringVar(&source, "source", "cache", "cache (getAppNames) or es (queryAppNames)")
	mark(cmd, risk.Read, "List application names")
	return cmd
}

func appsRequest(source string) (client.Request, error) {
	switch source {
	case "", "cache":
		return client.Request{Method: "POST", Path: "/getAppNames"}, nil
	case "es":
		return client.Request{Method: "POST", Path: "/queryAppNames", Body: dsl.AppNamesAgg()}, nil
	default:
		return client.Request{}, fmt.Errorf("--source must be cache or es")
	}
}

func bindSearchFlags(cmd *cobra.Command, f *searchFlags) {
	cmd.Flags().StringVar(&f.app, "app", "", "appName match_phrase; commas become spaces")
	cmd.Flags().StringVar(&f.appName, "app-name", "", "alias of --app")
	cmd.Flags().BoolVar(&f.excludeApp, "exclude-app", false, "put --app into bool.must_not instead of must")
	cmd.Flags().StringVar(&f.env, "env", "", "env match_phrase (commas are kept)")
	cmd.Flags().StringVar(&f.level, "level", "", "logLevel match_phrase")
	cmd.Flags().StringVar(&f.logLevel, "log-level", "", "alias of --level")
	cmd.Flags().StringVar(&f.traceID, "trace-id", "", "traceId match_phrase; switches sort to dtTime asc, seq asc")
	cmd.Flags().StringVar(&f.class, "class", "", "className match_phrase")
	cmd.Flags().StringVar(&f.className, "class-name", "", "alias of --class")
	cmd.Flags().StringVar(&f.server, "server", "", "serverName match_phrase")
	cmd.Flags().StringVar(&f.serverName, "server-name", "", "alias of --server")
	cmd.Flags().StringVarP(&f.content, "content", "q", "", "query_string on content; dotted identifiers auto-wrap as *value* (see --content-wildcard / --no-content-auto-wildcard)")
	cmd.Flags().BoolVar(&f.contentWildcard, "content-wildcard", false, "wrap bare -q as *value* for ES wildcard matching; structured query_string (AND/OR, field:value, quotes) is left unchanged")
	cmd.Flags().BoolVar(&f.noContentAutoWildcard, "no-content-auto-wildcard", false, "disable automatic *...* wrapping for dotted -q identifiers")
	cmd.Flags().StringArrayVar(&f.extend, "extend", nil, "repeatable field=value match_phrase for an extend/MDC field")
	cmd.Flags().StringVar(&f.fromTime, "from-time", "", "range dtTime gte and clientStartDate (ms or RFC3339)")
	cmd.Flags().StringVar(&f.toTime, "to-time", "", "range dtTime lt and clientEndDate; future values clamp to now")
	cmd.Flags().IntVar(&f.size, "size", 0, "page size (default 100, or config default_size)")
	cmd.Flags().IntVar(&f.from, "from", 0, "page offset")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "alias of --from")
	cmd.Flags().StringVar(&f.sort, "sort", "", "sort override: field:asc|desc[,...] or a JSON array")
	cmd.Flags().BoolVar(&f.noHighlight, "no-highlight", false, "omit the content highlight block")
	cmd.Flags().StringVar(&f.dsl, "dsl", "", "raw JSON body; conflicts with shortcut filters")
	cmd.Flags().StringVar(&f.dslFile, "dsl-file", "", "raw JSON body file; conflicts with shortcut filters")
	cmd.Flags().StringVar(&f.index, "index", "", "POST /query against this index (plume_log_run_YYYYMMDD); without time flags, window aligns to that day")
	cmd.Flags().StringVar(&f.rng, "range", "", "POST /query range: day, week, or month")
	cmd.Flags().BoolVar(&f.trace, "trace", false, "POST /clientQuery?trace=true")
}

func (a *App) planSearch(cmd *cobra.Command, f searchFlags, forceTrace bool, sizeFallback int) (client.Request, error) {
	app, err := oneOf("app", f.app, f.appName)
	if err != nil {
		return client.Request{}, err
	}
	level, err := oneOf("level", f.level, f.logLevel)
	if err != nil {
		return client.Request{}, err
	}
	className, err := oneOf("class", f.class, f.className)
	if err != nil {
		return client.Request{}, err
	}
	server, err := oneOf("server", f.server, f.serverName)
	if err != nil {
		return client.Request{}, err
	}
	f.app, f.level, f.class, f.server = app, level, className, server

	if err := validateRange(f.rng); err != nil {
		return client.Request{}, err
	}
	raw, hasRaw, err := loadOptionalDSL(f.dslFile, f.dsl)
	if err != nil {
		return client.Request{}, err
	}
	if hasRaw && hasShortcut(f) {
		return client.Request{}, errors.New("raw DSL conflicts with shortcut filters (--app, --env, --level, --trace-id, --class, --server, --content, --extend, --exclude-app, --sort, --no-highlight)")
	}
	if (f.trace || forceTrace) && (f.index != "" || f.rng != "") {
		return client.Request{}, errors.New("--trace selects clientQuery indices; omit --index and --range")
	}

	loc := time.Local
	explicitTime := strings.TrimSpace(f.fromTime) != "" || strings.TrimSpace(f.toTime) != "" || strings.TrimSpace(f.rng) != ""
	if strings.TrimSpace(f.index) != "" && !explicitTime {
		if fromAligned, toAligned, day, ok := dsl.IndexDayWindow(f.index, loc); ok {
			f.fromTime = strconv.FormatInt(fromAligned, 10)
			f.toTime = strconv.FormatInt(toAligned, 10)
			if cmd != nil {
				fmt.Fprint(cmd.ErrOrStderr(), dsl.FormatIndexDayNote(f.index, day))
			}
		} else if cmd != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: --index %q has no YYYYMMDD and no --from-time/--to-time/--range; server may use today's window and return 0 hits\n", f.index)
		}
	} else if strings.TrimSpace(f.index) != "" && strings.TrimSpace(f.rng) != "" && strings.TrimSpace(f.fromTime) == "" && strings.TrimSpace(f.toTime) == "" {
		if day, ok := dsl.ParseIndexDate(f.index, loc); ok {
			now := time.Now().In(loc)
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
			if !day.Equal(today) && cmd != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: --index %s is not today but --range %s is server-relative to now; prefer --from-time/--to-time for historical indexes\n", f.index, f.rng)
			}
		}
	}

	fromMS, toMS, err := dsl.ResolveWindow(f.fromTime, f.toTime, a.timeWindow, time.Now())
	if err != nil {
		return client.Request{}, err
	}

	var body []byte
	if hasRaw {
		body = raw
	} else {
		body, err = dsl.Build(dsl.Search{
			App:        f.app,
			ExcludeApp: f.excludeApp,
			Env:        f.env,
			Level:      f.level,
			TraceID:    f.traceID,
			ClassName:  f.class,
			ServerName: f.server,
			Content:         f.content,
			ContentWildcard:       f.contentWildcard,
			NoContentAutoWildcard: f.noContentAutoWildcard,
			Extend:          f.extend,
			FromMS:     fromMS,
			ToMS:       toMS,
			Sort:       f.sort,
			Highlight:  !f.noHighlight,
		})
		if err != nil {
			return client.Request{}, err
		}
	}

	q := url.Values{}
	size := a.defaultSize
	if sizeFallback > 0 {
		size = sizeFallback
	}
	if changed(cmd, "size") {
		size = f.size
	}
	if size < 0 {
		return client.Request{}, errors.New("--size must be >= 0")
	}
	from := 0
	fromSet, offsetSet := changed(cmd, "from"), changed(cmd, "offset")
	switch {
	case fromSet && offsetSet && f.from != f.offset:
		return client.Request{}, errors.New("--from and --offset differ")
	case offsetSet:
		from = f.offset
	case fromSet:
		from = f.from
	}
	if from < 0 {
		return client.Request{}, errors.New("--from must be >= 0")
	}
	q.Set("size", strconv.Itoa(size))
	q.Set("from", strconv.Itoa(from))

	path := "/clientQuery"
	if f.index != "" || f.rng != "" {
		path = "/query"
		if f.index != "" {
			q.Set("index", f.index)
		}
		if f.rng != "" {
			q.Set("range", f.rng)
		}
	} else {
		q.Set("clientStartDate", strconv.FormatInt(fromMS, 10))
		q.Set("clientEndDate", strconv.FormatInt(toMS, 10))
		if f.trace || forceTrace {
			q.Set("trace", "true")
		}
	}
	return client.Request{Method: "POST", Path: path, Query: q, Body: body}, nil
}

func hasShortcut(f searchFlags) bool {
	return strings.TrimSpace(f.app) != "" ||
		f.excludeApp ||
		strings.TrimSpace(f.env) != "" ||
		strings.TrimSpace(f.level) != "" ||
		strings.TrimSpace(f.traceID) != "" ||
		strings.TrimSpace(f.class) != "" ||
		strings.TrimSpace(f.server) != "" ||
		strings.TrimSpace(f.content) != "" ||
		len(f.extend) > 0 ||
		strings.TrimSpace(f.sort) != "" ||
		f.noHighlight
}

func oneOf(name, a, b string) (string, error) {
	if a != "" && b != "" && a != b {
		return "", fmt.Errorf("--%s was given twice with different values", name)
	}
	if a != "" {
		return a, nil
	}
	return b, nil
}

func validateRange(r string) error {
	switch r {
	case "", "day", "week", "month":
		return nil
	default:
		return fmt.Errorf("--range must be day, week, or month")
	}
}

func loadOptionalDSL(file, inline string) ([]byte, bool, error) {
	if file != "" && inline != "" {
		return nil, false, errors.New("pass only one of --dsl and --dsl-file")
	}
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, false, err
		}
		if !json.Valid(bytes.TrimSpace(b)) {
			return nil, false, fmt.Errorf("%s is not JSON", file)
		}
		return bytes.TrimSpace(b), true, nil
	}
	if strings.TrimSpace(inline) != "" {
		b := bytes.TrimSpace([]byte(inline))
		if !json.Valid(b) {
			return nil, false, errors.New("DSL is not JSON")
		}
		return b, true, nil
	}
	return nil, false, nil
}

func readDSLInput(cmd *cobra.Command, file, inline string, stdinOK bool) ([]byte, error) {
	b, ok, err := loadOptionalDSL(file, inline)
	if err != nil {
		return nil, err
	}
	if ok {
		return b, nil
	}
	if !stdinOK {
		return nil, errors.New("pass --file or --dsl")
	}
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok {
		st, err := f.Stat()
		if err == nil && st.Mode()&os.ModeCharDevice != 0 {
			return nil, errors.New("no DSL: pass --file or pipe JSON on stdin")
		}
	}
	raw, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, errors.New("no DSL: pass --file or pipe JSON on stdin")
	}
	if !json.Valid(raw) {
		return nil, errors.New("stdin is not JSON")
	}
	return raw, nil
}
