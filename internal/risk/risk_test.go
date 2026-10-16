package risk

import "testing"

func TestHighRiskGate(t *testing.T) {
	p := HighRisk(true, true)
	err := p.Check(false, "secret", "admin queue clear")
	if err == nil || !contains(err.Error(), "high-risk-write") {
		t.Fatalf("%v", err)
	}
	g, ok := err.(GateResult)
	if !ok || g.Action != "admin queue clear" || g.Hint == "" {
		t.Fatalf("%#v", err)
	}
	if !IsConfirmation(err) {
		t.Fatal("expected confirmation")
	}
	err = p.Check(true, "", "admin queue clear")
	if err == nil || !contains(err.Error(), "--admin-password") {
		t.Fatalf("%v", err)
	}
	if err := p.Check(true, "secret", "admin queue clear"); err != nil {
		t.Fatal(err)
	}
	if got := p.Missing(false, ""); len(got) != 2 {
		t.Fatalf("%v", got)
	}
	if err := ReadOnly().Check(false, "", "doctor"); err != nil {
		t.Fatal(err)
	}
	if ExitConfirmationRequired != 10 {
		t.Fatalf("exit code must be 10, got %d", ExitConfirmationRequired)
	}
}

func TestForAPI(t *testing.T) {
	q := ForAPI("POST", "/plumelog/deleteQueue")
	if q.Level != High || !q.NeedAdmin || !q.SendAdmin {
		t.Fatalf("%+v", q)
	}
	if ForAPI("POST", "/clientQuery").Level != Read {
		t.Fatal("clientQuery")
	}
	if ForAPI("GET", "/deleteIndex").Level != Read {
		t.Fatal("GET is not intercepted as a write")
	}
	if ForAPI("POST", "/deleteIndex").Level != High {
		t.Fatal("deleteIndex")
	}
	if ForAPI("POST", "/resetIndex").NeedAdmin {
		t.Fatal("resetIndex does not require admin password")
	}
}

func TestHintForMissing(t *testing.T) {
	if hintForMissing([]string{"--yes"}) != "add --yes to confirm" {
		t.Fatal("yes")
	}
	if !contains(hintForMissing([]string{"--admin-password"}), "--admin-password") {
		t.Fatal("admin")
	}
	h := hintForMissing([]string{"--yes", "--admin-password"})
	if !contains(h, "--yes") || !contains(h, "--admin-password") {
		t.Fatal(h)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
