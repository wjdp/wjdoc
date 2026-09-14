package render

import (
	"os"
	"testing"
)

func TestDetectModeFlagsWinOverTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	if got := DetectMode(os.Stdout, false, true); got != JSON {
		t.Errorf("jsonFlag: got %v, want JSON", got)
	}
	if got := DetectMode(os.Stdout, true, true); got != JSON {
		t.Errorf("both flags: got %v, want JSON (json wins)", got)
	}
	if got := DetectMode(os.Stdout, true, false); got != Plain {
		t.Errorf("plainFlag: got %v, want Plain", got)
	}
}

func TestDetectModeNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	if got := DetectMode(os.Stdout, false, false); got != Plain {
		t.Errorf("NO_COLOR set: got %v, want Plain", got)
	}
}

func TestDetectModeNonTerminalDefaultsPlain(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	f, err := os.CreateTemp(t.TempDir(), "notty")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if got := DetectMode(f, false, false); got != Plain {
		t.Errorf("non-terminal file: got %v, want Plain", got)
	}
}
