package core

import (
	"testing"

	"github.com/cyamoyed/proton/internal/plugin"
)

func TestTaskNeedsApproval(t *testing.T) {
	cases := []struct {
		name string
		task Task
		want bool
	}{
		{"no risk", Task{}, false},
		{"low risk", Task{Risk: "low"}, false},
		{"medium risk", Task{Risk: "medium"}, false},
		{"high risk", Task{Risk: "high"}, true},
		{"high risk case-insensitive", Task{Risk: "High"}, true},
		{"confirm flag", Task{Confirm: true}, true},
		{"confirm with low risk", Task{Risk: "low", Confirm: true}, true},
	}
	for _, c := range cases {
		if got := TaskNeedsApproval(c.task); got != c.want {
			t.Errorf("%s: TaskNeedsApproval = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMissingRequiredTools(t *testing.T) {
	task := Task{Checks: []plugin.EnvCheck{
		{Name: "Present", Check: "echo ok", Required: true},
		{Name: "OptionalMissing", Check: "proton-totally-missing-tool --version", Required: false},
		{Name: "RequiredMissing", Check: "proton-totally-missing-tool --version", Required: true},
	}}
	missing := MissingRequiredTools(task)
	if len(missing) != 1 || missing[0] != "RequiredMissing" {
		t.Errorf("MissingRequiredTools = %v, want [RequiredMissing]", missing)
	}
}
