package envvar

import "testing"

func TestCurrentNameWins(t *testing.T) {
	t.Setenv("TENDRIL_THING", "new")
	t.Setenv("OPENTENDRIL_THING", "old")
	if got := Lookup("TENDRIL_THING", "OPENTENDRIL_THING"); got != "new" {
		t.Fatalf("got %q, want the current name's value", got)
	}
}

// A superseded name must still work. Silently ignoring it loses configuration an
// operator believes is set.
func TestSupersededNameIsStillHonoured(t *testing.T) {
	t.Setenv("TENDRIL_OTHER", "")
	t.Setenv("OPENTENDRIL_OTHER", "old")
	if got := Lookup("TENDRIL_OTHER", "OPENTENDRIL_OTHER"); got != "old" {
		t.Fatalf("got %q, want the superseded name's value", got)
	}
}

func TestNeitherSetIsEmpty(t *testing.T) {
	t.Setenv("TENDRIL_ABSENT", "")
	t.Setenv("OPENTENDRIL_ABSENT", "")
	if got := Lookup("TENDRIL_ABSENT", "OPENTENDRIL_ABSENT"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
