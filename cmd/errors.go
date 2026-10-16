package cmd

// PrintedError means the ok-envelope was already written to stdout.
type PrintedError struct {
	Msg string
}

func (e *PrintedError) Error() string {
	if e == nil || e.Msg == "" {
		return "command failed"
	}
	return e.Msg
}
