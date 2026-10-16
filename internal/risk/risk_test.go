package risk

import "testing"

func TestHighRiskGate(t *testing.T) {
	p := HighRisk(true, true)
	if err := p.Check(false, "secret"); err == nil || !contains(err.Error(), "high-risk-write") {
		t.Fatalf("%v", err)
	}
	if err := p.Check(true, ""); err == nil || !contains(err.Error(), "--admin-password") {
		t.Fatalf("%v", err)
	}
	if err := p.Check(true, "secret"); err != nil {
		t.Fatal(err)
	}
	if got := p.Missing(false, ""); len(got) != 2 {
		t.Fatalf("%v", got)
	}
	if err := ReadOnly().Check(false, ""); err != nil {
		t.Fatal(err)
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
