package cmd

// PrintedError means the ok-envelope was already written to stdout.
type PrintedError struct {
	Msg  string
	Code int // 0 means exit 1; confirmation gate uses risk.ExitConfirmationRequired (10)
}

func (e *PrintedError) Error() string {
	if e == nil || e.Msg == "" {
		return "command failed"
	}
	return e.Msg
}

func (e *PrintedError) ExitCode() int {
	if e == nil || e.Code == 0 {
		return 1
	}
	return e.Code
}
