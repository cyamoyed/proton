package envcheck

import (
	"testing"

	"github.com/fupeijun/proton/internal/plugin"
)

func TestCheckEnvironment(t *testing.T) {
	checks := []plugin.EnvCheck{
		{Name: "go", Check: "go version", Required: true},
		{Name: "nonexistent", Check: "nonexistent --version", Required: false},
	}

	statuses := CheckEnvironment(checks)
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	// Go should be found (assuming Go is installed)
	if statuses[0].Name == "go" && !statuses[0].OK {
		t.Error("go should be available")
	}

	// Nonexistent should not be found
	if statuses[1].Name == "nonexistent" && statuses[1].OK {
		t.Error("nonexistent should not be available")
	}
}
