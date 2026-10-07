package organism

import "testing"

// TestEveryHealthSourceIsListedAndLabelled: the list is what the panel walks
// and what the manager test checks for coverage, so a source missing from it
// is one that silently never appears.
func TestEveryHealthSourceIsListedAndLabelled(t *testing.T) {
	if got, want := len(AllHealthSources), int(healthSourceCount); got != want {
		t.Fatalf("AllHealthSources has %d entries, there are %d sources", got, want)
	}
	seen := map[HealthSource]bool{}
	labels := map[string]HealthSource{}
	for _, src := range AllHealthSources {
		if seen[src] {
			t.Errorf("source %d is listed twice", src)
		}
		seen[src] = true
		label := src.Label()
		if label == "" {
			t.Errorf("source %d has no label", src)
			continue
		}
		if other, dup := labels[label]; dup {
			t.Errorf("sources %d and %d share the label %q", other, src, label)
		}
		labels[label] = src
	}
	for s := HealthSource(0); s < healthSourceCount; s++ {
		if !seen[s] {
			t.Errorf("source %d (%q) is not in AllHealthSources", s, s.Label())
		}
	}
}

func TestLedgerAccumulatesAndTotals(t *testing.T) {
	o := &Organism{}
	// Nothing resolved yet, so the panel can tell this apart from a free cycle.
	if o.HealthLedger().Recorded {
		t.Error("a fresh organism reports a recorded ledger")
	}
	o.ResetHealthLedger()
	// Several attacks can land on one organism in a cycle.
	o.RecordHealth(HealthFromAttack, -3)
	o.RecordHealth(HealthFromAttack, -2)
	o.RecordHealth(HealthFromChemo, 4.5)
	l := o.HealthLedger()
	if !l.Recorded {
		t.Error("the ledger is not marked recorded after a reset")
	}
	if got := l.Amounts[HealthFromAttack]; got != -5 {
		t.Errorf("two hits recorded %v, want -5", got)
	}
	if got := l.Total(); got != -0.5 {
		t.Errorf("total %v, want -0.5", got)
	}
	o.ResetHealthLedger()
	if got := o.HealthLedger().Total(); got != 0 {
		t.Errorf("the reset left %v behind", got)
	}
}

// TestRecordingAnUnknownSourceIsIgnored: the ledger is observational, so a
// bad index must not panic in the resolve loop.
func TestRecordingAnUnknownSourceIsIgnored(t *testing.T) {
	o := &Organism{}
	o.ResetHealthLedger()
	o.RecordHealth(HealthSource(-1), 5)
	o.RecordHealth(healthSourceCount, 5)
	if got := o.HealthLedger().Total(); got != 0 {
		t.Errorf("an out-of-range source recorded %v", got)
	}
	if got := HealthSource(99).Label(); got != "" {
		t.Errorf("an out-of-range source labelled %q", got)
	}
}
