// Completion requires consistent search settings, counts, and error records.
// A zero exit code cannot compensate for missing evidence.
package nativecheck

import (
	"math"
	"strings"
	"testing"
)

func TestCompletedReport(t *testing.T) {
	result, err := assess(Request{Minimum: 13, Maximum: 23}, completedFixture(t))
	if err != nil || result.Status != SearchReportedComplete || result.Counterexample != nil {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
}

func TestRejectChangedReport(t *testing.T) {
	for _, change := range [][2]string{
		{"count=11", "count=10"},
		{"count=11", "count=18446744073709551616"},
		{"overflow=0", "overflow=1"},
		{"OBSERVATIONS", "MISSING"},
		{"State-vector", "MISSING"},
		{"errors: 0", "errors: nope"},
		{"errors: 0", "errors: 1"},
		{"Full statespace", "Bit statespace"},
		{"Full statespace", "Hash-Compact 4"},
		{"assertion violations\t+", "assertion violations\t-"},
		{"(none specified)", "(other claim)"},
		{"invalid end states\t+", "invalid end states\t-"},
		{"(disabled by -DSAFETY)", "(other search)"},
	} {
		search := completedFixture(t)
		search.Output = strings.ReplaceAll(search.Output, change[0], change[1])
		if _, err := assess(Request{Minimum: 13, Maximum: 23}, search); err == nil {
			t.Errorf("accepted mutation %q", change)
		}
	}
}

func TestRejectDuplicateEvidence(t *testing.T) {
	for _, record := range []string{
		"OBSERVATIONS count=11 overflow=0\n",
		"State-vector 20 byte, depth reached 18, errors: 0\n",
		"Full statespace search for:\n",
		"COUNTEREXAMPLE input=23 output=29\n",
	} {
		search := completedFixture(t)
		search.Output += record
		if _, err := assess(Request{Minimum: 13, Maximum: 23}, search); err == nil {
			t.Errorf("accepted extra record %q", record)
		}
	}
}

func TestFullWidthCountCannotEstablishCompletion(t *testing.T) {
	if _, err := assess(Request{Maximum: math.MaxUint64}, completedFixture(t)); err == nil {
		t.Fatal("accepted a full-width interval without a representable observation count")
	}
}
