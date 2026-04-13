package shareCmd

import (
	"testing"
)

func TestFmtTime_Zero(t *testing.T) {
	if got := fmtTime(0); got != "-" {
		t.Fatalf("fmtTime(0) = %q, want %q", got, "-")
	}
}

func TestFmtTime_NonZero(t *testing.T) {
	got := fmtTime(1705276800)
	// Just verify it produces a date-like string, not "-".
	if got == "-" {
		t.Fatal("fmtTime(nonzero) should not return '-'")
	}
	if len(got) != 10 { // "YYYY-MM-DD"
		t.Fatalf("fmtTime(1705276800) = %q, expected YYYY-MM-DD format", got)
	}
}
