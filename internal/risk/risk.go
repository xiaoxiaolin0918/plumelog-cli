// Package risk classifies commands and enforces the high-risk gate.
package risk

import "fmt"

const (
	// Read is a non-mutating call.
	Read = "read"
	// Write is a mutation that does not require --yes.
	Write = "write"
	// High is a high-risk-write. Execution requires --yes, and some
	// commands also require --admin-password.
	High = "high-risk-write"
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
func (p Policy) Check(yes bool, adminPassword string) error {
	missing := p.Missing(yes, adminPassword)
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("refusing %s without %s", p.Level, join(missing))
}

func join(parts []string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + " and " + parts[1]
}
