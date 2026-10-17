// Package cmd is the plumelog cobra command tree.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/client"
	"plumelog-cli/internal/config"
	"plumelog-cli/internal/dsl"
	"plumelog-cli/internal/output"
	"plumelog-cli/internal/risk"
	"plumelog-cli/internal/version"
)

// App is one CLI invocation. Tests build a fresh one via NewRoot.
type App struct {
	flagBaseURL       string
	flagFormat        string
	flagJQ            string
	flagDryRun        bool
	flagYes           bool
	flagAutoLogin     bool
	flagAdminPassword string
	flagPathPrefix    string
	flagCookieJar     string
	flagConfig        string
	flagTimeout       time.Duration

	baseURL       string
	format        string
	jq            string
	dryRun        bool
	yes           bool
	autoLogin     bool
	adminPassword string
	prefixMode    string
	timeout       time.Duration
	configPath    string
	cookiePath    string
	cfg           config.File
	client        *client.Client
	jar           *client.Jar
	defaultSize   int
	timeWindow    time.Duration
	extraSecrets  []string
}

// Main runs the CLI and returns an exit code.
func Main() int {
	root := NewRoot()
	err := root.Execute()
	if err == nil {
		return 0
	}
	var pe *PrintedError
	if errors.As(err, &pe) {
		return pe.ExitCode()
	}
	fmt.Fprintln(os.Stderr, "Error:", err)
	return 1
}

// NewRoot builds the command tree described by the architecture contract.
func NewRoot() *cobra.Command {
	a := &App{}
	root := &cobra.Command{
		Use:   "plumelog",
		Short: "PlumeLog server CLI (session cookies, not bearer tokens)",
		Long: `plumelog talks to a PlumeLog server over HTTP.

Authentication is the same servlet session the web UI uses (usually a
JSESSIONID cookie). There is no bearer token. Cookies are stored in a
Netscape cookies.txt file and are never printed.

Config: ~/.config/plumelog/config.json (Windows: %APPDATA%\plumelog\config.json).
Precedence: flags > environment > config.json > defaults.

Quick start:
  plumelog doctor
  plumelog auth login --username admin --password "$PLUMELOG_LOGIN_PASSWORD"
  plumelog query search --app demo --level ERROR -q timeout
  plumelog trace get --trace-id TRACE_ID
  plumelog api POST /getRunModel

Risk:
  read             runs immediately
  write            runs immediately; --dry-run prints the request instead
  high-risk-write  requires --yes to execute
                   admin queue clear and admin index delete also require
                   --admin-password (sent as query adminPassWord, never printed)
  --dry-run        prints Method, URL, redacted headers, and body; does not send

Output:
  success          JSON on stdout: {ok:true, result:...}
  failure          JSON on stderr: {ok:false, error:{type,message,...}}
  confirmation     missing --yes -> exit 10 + error.subtype=confirmation_required (stderr)
                   (not exit 100; error is an object, not a string)

Discovery:
  plumelog --help
  plumelog query --help
  plumelog skills list
  plumelog <command> --help`,
		Example: `  plumelog doctor
  plumelog auth status --probe
  plumelog whoami
  plumelog query search --app order --level ERROR --from-time 1700000000000 --to-time 1700003600000
  plumelog admin queue clear --dry-run`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Version,
	}
	root.SetVersionTemplate("{\"ok\":true,\"result\":{\"name\":\"plumelog\",\"version\":\"{{.Version}}\"}}\n")
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentPreRunE = a.resolve

	p := root.PersistentFlags()
	p.StringVar(&a.flagBaseURL, "base-url", "", "server root, no trailing slash (env PLUMELOG_BASE_URL, default http://localhost:8891)")
	p.StringVar(&a.flagFormat, "format", "", "output format: json, pretty, or table (default json)")
	p.StringVar(&a.flagJQ, "jq", "", "jq expression applied to the JSON envelope")
	p.BoolVar(&a.flagDryRun, "dry-run", false, "print the request and do not send it")
	p.BoolVarP(&a.flagYes, "yes", "y", false, "confirm a high-risk-write")
	p.BoolVar(&a.flagAutoLogin, "auto-login", false, "on 401, silent login once from PLUMELOG_LOGIN_* env and replay (env PLUMELOG_AUTO_LOGIN=1)")
	p.StringVar(&a.flagAdminPassword, "admin-password", "", "admin password for deleteQueue/deleteIndex (env PLUMELOG_ADMIN_PASSWORD); never printed")
	p.StringVar(&a.flagPathPrefix, "path-prefix", "auto", `path prefix: auto, "" (or root), or /plumelog`)
	p.StringVar(&a.flagCookieJar, "cookie-jar", "", "override the Netscape cookie jar path")
	p.StringVar(&a.flagConfig, "config", "", "override the config.json path")
	p.DurationVar(&a.flagTimeout, "timeout", 0, "HTTP timeout (default 30s)")

	root.AddCommand(
		a.newAuthCmd(),
		a.newWhoamiCmd(),
		a.newDoctorCmd(),
		a.newConfigCmd(),
		a.newQueryCmd(),
		a.newTraceCmd(),
		a.newAppCmd(),
		a.newAlarmCmd(),
		a.newAdminCmd(),
		a.newAPICmd(),
		a.newSkillsCmd(),
		a.newVersionCmd(),
	)
	silence(root)
	return root
}

func silence(cmd *cobra.Command) {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	for _, c := range cmd.Commands() {
		silence(c)
	}
}

func helpOnly(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func mark(cmd *cobra.Command, level, short string) {
	cmd.Short = short + " (Risk: " + level + ")"
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["risk"] = level
	note := "Risk: " + level + "."
	switch level {
	case risk.High:
		note += "\nExecution requires --yes. --dry-run prints the request and does not send it, even when --yes is set."
	case risk.Write:
		note += "\n--dry-run prints the request and does not send it."
	}
	if strings.TrimSpace(cmd.Long) == "" {
		cmd.Long = note
	} else {
		cmd.Long = strings.TrimRight(cmd.Long, "\n") + "\n\n" + note
	}
}

func (a *App) resolve(cmd *cobra.Command, _ []string) error {
	if cmd.Name() == "version" || isSkillsCommand(cmd) {
		a.format = "json"
		if a.flagFormat != "" {
			a.format = a.flagFormat
		}
		a.jq = a.flagJQ
		a.dryRun = a.flagDryRun
		return nil
	}
	if err := a.resolveInto(cmd); err != nil {
		a.format = "json"
		return a.fail(cmd, "", err)
	}
	return nil
}

func (a *App) resolveInto(cmd *cobra.Command) error {
	defCfg, defCookie, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	a.configPath = defCfg
	if changed(cmd, "config") && strings.TrimSpace(a.flagConfig) != "" {
		a.configPath = a.flagConfig
	}
	file, _, err := config.Load(a.configPath)
	if err != nil {
		return err
	}
	a.cfg = file

	base := "http://localhost:8891"
	if file.BaseURL != "" {
		base = file.BaseURL
	}
	if v := os.Getenv("PLUMELOG_BASE_URL"); v != "" {
		base = v
	}
	if changed(cmd, "base-url") {
		base = a.flagBaseURL
	}
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" || !strings.Contains(base, "://") {
		return fmt.Errorf("base-url must look like http://localhost:8891")
	}
	a.baseURL = base

	mode := "auto"
	if file.PathPrefix != nil {
		mode = *file.PathPrefix
	}
	if v := os.Getenv("PLUMELOG_PATH_PREFIX"); v != "" {
		mode = v
	}
	if changed(cmd, "path-prefix") {
		mode = a.flagPathPrefix
	}
	mode, err = normalizePrefix(mode)
	if err != nil {
		return err
	}
	a.prefixMode = mode
	prefer := ""
	if file.PathPrefix != nil {
		prefer = *file.PathPrefix
	}
	var prefixes []string
	switch {
	case mode == "auto" && prefer == "/plumelog":
		prefixes = []string{"/plumelog", ""}
	case mode == "auto":
		prefixes = []string{"", "/plumelog"}
	default:
		prefixes = []string{mode}
	}

	format := "json"
	if file.Format != "" {
		format = file.Format
	}
	if changed(cmd, "format") {
		format = a.flagFormat
	}
	switch format {
	case "json", "pretty", "table":
	default:
		return fmt.Errorf("--format %q must be json, pretty, or table", format)
	}
	a.format = format
	a.jq = a.flagJQ
	a.dryRun = a.flagDryRun
	a.yes = a.flagYes
	a.autoLogin = a.flagAutoLogin || envTruthy("PLUMELOG_AUTO_LOGIN")

	admin := os.Getenv("PLUMELOG_ADMIN_PASSWORD")
	if changed(cmd, "admin-password") {
		admin = a.flagAdminPassword
	}
	a.adminPassword = admin
	a.track(admin)

	timeout := 30 * time.Second
	if file.TimeoutSec > 0 {
		timeout = time.Duration(file.TimeoutSec) * time.Second
	}
	if changed(cmd, "timeout") && a.flagTimeout > 0 {
		timeout = a.flagTimeout
	}
	a.timeout = timeout

	a.defaultSize = 100
	if file.DefaultSize > 0 {
		a.defaultSize = file.DefaultSize
	}
	a.timeWindow, err = dsl.ParseWindow(file.DefaultTimeWindow)
	if err != nil {
		return err
	}

	a.cookiePath = defCookie
	if file.CookieJar != "" {
		a.cookiePath = file.CookieJar
	}
	if changed(cmd, "cookie-jar") && strings.TrimSpace(a.flagCookieJar) != "" {
		a.cookiePath = a.flagCookieJar
	}
	jar, err := client.Open(a.cookiePath)
	if err != nil {
		return err
	}
	a.jar = jar
	hc := clientHTTP(timeout, jar)
	a.client = &client.Client{
		BaseURL:  a.baseURL,
		Prefixes: prefixes,
		HTTP:     &hc,
		Jar:      jar,
	}
	return nil
}

func normalizePrefix(v string) (string, error) {
	switch strings.TrimSpace(v) {
	case "", "root":
		return "", nil
	case "auto":
		return "auto", nil
	case "/plumelog":
		return "/plumelog", nil
	default:
		return "", fmt.Errorf(`--path-prefix %q must be auto, "" (or root), or /plumelog`, v)
	}
}

func changed(cmd *cobra.Command, name string) bool {
	if cmd.Flags().Changed(name) {
		return true
	}
	if cmd.Root() != nil && cmd.Root().PersistentFlags().Changed(name) {
		return true
	}
	return false
}

func (a *App) track(secret string) {
	if secret != "" {
		a.extraSecrets = append(a.extraSecrets, secret)
	}
}

func (a *App) secrets() []string {
	out := append([]string{}, a.extraSecrets...)
	if a.adminPassword != "" {
		out = append(out, a.adminPassword)
	}
	if a.jar != nil {
		out = append(out, a.jar.SecretValues()...)
	}
	return out
}

func (a *App) scrub(s string) string {
	return string(output.Scrub([]byte(s), a.secrets()))
}

func (a *App) write(cmd *cobra.Command, env output.Envelope) error {
	if a.format == "" {
		a.format = "json"
	}
	if env.Error != nil {
		env.Error = output.ScrubError(env.Error, a.scrub)
	}
	if env.Hint != "" {
		env.Hint = a.scrub(env.Hint)
	}
	if env.URL != "" {
		env.URL = client.RedactURL(env.URL)
	}
	for i, u := range env.FallbackURLs {
		env.FallbackURLs[i] = client.RedactURL(u)
	}
	// Success -> stdout; failure -> stderr (yunxiao-aligned).
	w := cmd.OutOrStdout()
	if !env.OK {
		w = cmd.ErrOrStderr()
	}
	return output.Write(w, a.format, a.jq, env, a.secrets())
}

func (a *App) fail(cmd *cobra.Command, level string, err error) error {
	body := output.ErrorBody{Type: "cli", Message: err.Error()}
	if level != "" {
		body.Risk = level
	}
	env := output.Envelope{OK: false, Error: body}
	if level != "" {
		env.Risk = level
	}
	if werr := a.write(cmd, env); werr != nil {
		return werr
	}
	return &PrintedError{Msg: body.Message}
}

func (a *App) failConfirmation(cmd *cobra.Command, g risk.GateResult) error {
	body := output.ErrorBody{
		Type:    "confirmation",
		Subtype: "confirmation_required",
		Message: g.Error(),
		Hint:    g.Hint,
		Risk:    g.Risk,
		Action:  g.Action,
	}
	env := output.Envelope{OK: false, Error: body, Risk: g.Risk, Hint: g.Hint}
	if werr := a.write(cmd, env); werr != nil {
		return werr
	}
	return &PrintedError{Msg: body.Message, Code: risk.ExitConfirmationRequired}
}

func commandAction(cmd *cobra.Command) string {
	path := strings.TrimSpace(cmd.CommandPath())
	return strings.TrimSpace(strings.TrimPrefix(path, "plumelog"))
}

func (a *App) ok(cmd *cobra.Command, v any) error {
	return a.write(cmd, output.Envelope{OK: true, Result: output.MustResult(v)})
}

func (a *App) prepare(cmd *cobra.Command, policy risk.Policy, req *client.Request) (bool, error) {
	if policy.SendAdmin && a.adminPassword != "" {
		if req.Query == nil {
			req.Query = url.Values{}
		}
		req.Query.Set("adminPassWord", a.adminPassword)
	}
	if a.dryRun {
		env, err := a.client.Preview(*req, a.cookieNames(), policy.Missing(a.yes, a.adminPassword), policy.Level)
		if err != nil {
			return true, a.fail(cmd, policy.Level, err)
		}
		if err := a.write(cmd, env); err != nil {
			return true, err
		}
		return true, nil
	}
	if err := policy.Check(a.yes, a.adminPassword, commandAction(cmd)); err != nil {
		if g, ok := err.(risk.GateResult); ok {
			return true, a.failConfirmation(cmd, g)
		}
		return true, a.fail(cmd, policy.Level, err)
	}
	return false, nil
}

func (a *App) invoke(cmd *cobra.Command, policy risk.Policy, req client.Request) error {
	stop, err := a.prepare(cmd, policy, &req)
	if stop || err != nil {
		return err
	}
	resp, err := a.do(cmd, req)
	if err != nil {
		return err
	}
	if a.autoLogin && !a.dryRun && isUnauthorized(resp) && !isLoginRequest(req) {
		if err := a.silentLogin(cmd); err != nil {
			return err
		}
		resp, err = a.do(cmd, req)
		if err != nil {
			return err
		}
	}
	return a.finishUpstream(cmd, resp)
}

func (a *App) do(cmd *cobra.Command, req client.Request) (client.Response, error) {
	if a.client == nil {
		return client.Response{}, a.fail(cmd, "", errors.New("HTTP client is not configured"))
	}
	resp, err := a.client.Do(cmd.Context(), req)
	if err != nil {
		return client.Response{}, a.fail(cmd, "", fmt.Errorf("request failed: %v", err))
	}
	return resp, nil
}

func (a *App) finishUpstream(cmd *cobra.Command, resp client.Response) error {
	if code, ok := client.ResultCode(resp.Body); ok && code >= 400 {
		hint := ""
		switch code {
		case 401:
			hint = "not logged in; run: plumelog auth login --username USER"
		case 402:
			hint = "invalid username or password"
		}
		env := output.Envelope{
			OK: false,
			Error: output.ErrorBody{
				Type:    "api",
				Message: fmt.Sprintf("server returned code %d", code),
				Hint:    hint,
			},
			URL:    client.RedactURL(resp.FinalURL),
			Result: output.RawResult(resp.Body),
			Hint:   hint,
		}
		if err := a.write(cmd, env); err != nil {
			return err
		}
		return &PrintedError{Msg: output.ErrorMessage(env.Error)}
	}
	if resp.Status >= 400 {
		return a.fail(cmd, "", fmt.Errorf("HTTP %d from %s", resp.Status, client.RedactURL(resp.FinalURL)))
	}
	env := output.Envelope{
		OK:     true,
		URL:    client.RedactURL(resp.FinalURL),
		Result: output.RawResult(resp.Body),
	}
	if len(strings.TrimSpace(string(resp.Body))) == 0 {
		env.Hint = "server returned an empty body (often no matching index)"
	}
	return a.write(cmd, env)
}

func envTruthy(name string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func isUnauthorized(resp client.Response) bool {
	if code, ok := client.ResultCode(resp.Body); ok && code == 401 {
		return true
	}
	return resp.Status == 401
}

func isLoginRequest(req client.Request) bool {
	p := strings.TrimSuffix(req.Path, "/")
	return p == "/login" || strings.HasSuffix(p, "/login")
}

// silentLogin POSTs /login once using env/config credentials. Password is never printed.
func (a *App) silentLogin(cmd *cobra.Command) error {
	user := strings.TrimSpace(os.Getenv("PLUMELOG_LOGIN_USERNAME"))
	if user == "" {
		user = strings.TrimSpace(a.cfg.LoginUsername)
	}
	pass := os.Getenv("PLUMELOG_LOGIN_PASSWORD")
	if user == "" || pass == "" {
		return a.fail(cmd, risk.Write, errors.New("auto-login requires PLUMELOG_LOGIN_USERNAME and PLUMELOG_LOGIN_PASSWORD"))
	}
	a.track(pass)
	body, err := json.Marshal(map[string]string{"username": user, "password": pass})
	if err != nil {
		return a.fail(cmd, risk.Write, err)
	}
	if a.client == nil {
		return a.fail(cmd, risk.Write, errors.New("HTTP client is not configured"))
	}
	resp, err := a.client.Do(cmd.Context(), client.Request{Method: "POST", Path: "/login", Body: body})
	if err != nil {
		return a.fail(cmd, risk.Write, fmt.Errorf("auto-login failed: %v", err))
	}
	if code, ok := client.ResultCode(resp.Body); ok && code == 402 {
		_ = a.jar.Clear()
		return a.fail(cmd, risk.Write, errors.New("auto-login invalid credentials (402)"))
	}
	if code, ok := client.ResultCode(resp.Body); ok && code >= 400 {
		return a.fail(cmd, risk.Write, fmt.Errorf("auto-login server returned code %d", code))
	}
	if resp.Status >= 400 {
		return a.fail(cmd, risk.Write, fmt.Errorf("auto-login HTTP %d", resp.Status))
	}
	return nil
}

func (a *App) cookieNames() []string {
	if a.jar == nil || a.baseURL == "" {
		return nil
	}
	u, err := url.Parse(a.baseURL + "/")
	if err != nil {
		return nil
	}
	return a.jar.Names(u)
}
