package style

import (
	"strings"
	"testing"
)

func TestTypeLabelHasText(t *testing.T) {
	got := TypeLabel("fire")
	if got == "" {
		t.Fatal("empty")
	}
	if !strings.Contains(got, "Fire") {
		t.Fatalf("missing Fire in %q", got)
	}
}

func TestStatusMarkers(t *testing.T) {
	if !strings.Contains(StatusError("boom"), "[ERROR]") {
		t.Fatal("error marker")
	}
	if !strings.Contains(StatusWarn("slow"), "[!]") {
		t.Fatal("warn marker")
	}
	if !strings.Contains(StatusLoading(""), "Loading") {
		t.Fatal("loading text")
	}
}
