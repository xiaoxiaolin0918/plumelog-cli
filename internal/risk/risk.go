// Package risk classifies commands and enforces the high-risk gate.
package risk

import (
	"fmt"
	"strings"
)

const (
	// Read is a non-mutating call.
	Read = "read"
	// Write is a mutation that does not require --yes.
	Write = "write"
	// High is a high-risk-write. Execution requires --yes, and some
	// commands also require --admin-password.
	High = "high-risk-write"

	// ExitConfirmationRequired matches yunxiao: missing --yes / admin gate.
	ExitConfirmationRequired = 10
)

// Policy is the gate for one command invocation.
type Policy struct {
	Level     string
	NeedYes   bool
	NeedAdmin bool
	// SendAdmin places adminPassWord on the query string. Only deleteQueue
	// and deleteIndex read that parameter upstream.
	SendAdmin bool
}

// GateResult is returned when a high-risk-write lacks confirmation flags.
// Agents should treat exit 10 + subtype confirmation_required like yunxiao.
type GateResult struct {
	Action  string
	Risk    string
	Missing []string
	Hint    string
}

func (g GateResult) Error() string {
	return fmt.Sprintf("refusing %s without %s", g.Risk, join(g.Missing))
}

// IsConfirmation reports whether err is a confirmation gate.
func IsConfirmation(err error) bool {
	_, ok := err.(GateResult)
	return ok
}

// ReadOnly is the policy for read commands.
func ReadOnly() Policy { return Policy{Level: Read} }

// Mutation is a write that does not require --yes.
func Mutation() Policy { return Policy{Level: Write} }

// HighRisk builds a high-risk-write policy.
func HighRisk(needAdmin, sendAdmin bool) Policy {
	return Policy{Level: High, NeedYes: true, NeedAdmin: needAdmin, SendAdmin: sendAdmin}
}

// Missing lists flags still required before a real execution.
// Dry-run uses this list and does not fail.
func (p Policy) Missing(yes bool, adminPassword string) []string {
	if p.Level != High {
		return nil
	}
	var out []string
	if p.NeedYes && !yes {
		out = append(out, "--yes")
	}
	if p.NeedAdmin && adminPassword == "" {
		out = append(out, "--admin-password")
	}
	return out
}

// Check blocks a real execution that is missing confirmation.
// action is the agent-facing command path (e.g. "admin queue clear").
func (p Policy) Check(yes bool, adminPassword, action string) error {
	missing := p.Missing(yes, adminPassword)
	if len(missing) == 0 {
		return nil
	}
	return GateResult{
		Action:  action,
		Risk:    p.Level,
		Missing: missing,
		Hint:    hintForMissing(missing),
	}
}

func hintForMissing(missing []string) string {
	hasYes, hasAdmin := false, false
	for _, m := range missing {
		switch m {
		case "--yes":
			hasYes = true
		case "--admin-password":
			hasAdmin = true
		}
	}
	switch {
	case hasYes && hasAdmin:
		return "add --yes to confirm; also pass --admin-password (sent as adminPassWord, never printed)"
	case hasAdmin:
		return "pass --admin-password (sent as adminPassWord, never printed)"
	default:
		return "add --yes to confirm"
	}
}

func join(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}
