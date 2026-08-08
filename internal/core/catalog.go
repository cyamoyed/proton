package core

import (
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/cyamoyed/proton/internal/plugin"
)

// Task is the stable, machine-readable representation of a command Proton can
// execute. Project is relative to the scan root and ProjectDir is absolute.
type Task struct {
	ID          string            `json:"id"`
	Project     string            `json:"project"`
	ProjectDir  string            `json:"project_dir"`
	Types       []string          `json:"types"`
	Confidence  int               `json:"confidence"`
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Description string            `json:"description,omitempty"`
	Source      string            `json:"source"`
	Risk        string            `json:"risk,omitempty"`
	Confirm     bool              `json:"confirm,omitempty"`
	Required    []string          `json:"required_tools,omitempty"`
	Checks      []plugin.EnvCheck `json:"-"`
}

// Catalog flattens detected projects into tasks with deterministic IDs.
func Catalog(discovery Discovery) []Task {
	var tasks []Task
	usedIDs := make(map[string]int)
	for _, project := range discovery.Projects {
		relative, err := filepath.Rel(discovery.Root, project.Dir)
		if err != nil || relative == "" {
			relative = "."
		}
		relative = filepath.ToSlash(relative)
		projectID := relative
		if projectID == "." {
			projectID = "root"
		}
		for index, command := range project.Plugin.Commands {
			checks := command.Checks
			if checks == nil {
				checks = project.Plugin.Env
			}
			required := requiredTools(checks)
			nameID := identifier(command.Name)
			if nameID == "" {
				nameID = "task-" + strconv.Itoa(index+1)
			}
			baseID := projectID + ":" + nameID
			usedIDs[baseID]++
			id := baseID
			if usedIDs[baseID] > 1 {
				id += "-" + strconv.Itoa(usedIDs[baseID])
			}
			source := command.Source
			if source == "" {
				source = project.Plugin.Source
			}
			confidence := command.Confidence
			if confidence == 0 {
				confidence = project.Confidence
			}
			tasks = append(tasks, Task{
				ID:          id,
				Project:     relative,
				ProjectDir:  project.Dir,
				Types:       append([]string(nil), project.Types...),
				Confidence:  confidence,
				Name:        command.Name,
				Command:     command.Run,
				Description: command.Description,
				Source:      source,
				Risk:        command.Risk,
				Confirm:     command.Confirm,
				Required:    append([]string(nil), required...),
				Checks:      append([]plugin.EnvCheck(nil), checks...),
			})
		}
	}
	return tasks
}

// FindTask resolves an exact task ID.
func FindTask(tasks []Task, id string) (Task, bool) {
	for _, task := range tasks {
		if task.ID == id {
			return task, true
		}
	}
	return Task{}, false
}

func identifier(value string) string {
	var result strings.Builder
	separator := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if separator && result.Len() > 0 {
				result.WriteByte('-')
			}
			result.WriteRune(r)
			separator = false
			continue
		}
		separator = true
	}
	return result.String()
}

func requiredTools(checks []plugin.EnvCheck) []string {
	var names []string
	for _, check := range checks {
		if check.Required {
			names = append(names, check.Name)
		}
	}
	return names
}
