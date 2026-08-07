package detector

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fupeijun/proton/internal/plugin"
)

type DetectedProject struct {
	Plugin     *plugin.Plugin
	Types      []string
	Confidence int
	Dir        string
}

const maxDepth = 3

func DetectProject(dir string, plugins []*plugin.Plugin) []*DetectedProject {
	return DetectProjectWithDepth(dir, plugins, maxDepth)
}

// DetectProjectWithDepth detects and groups all technologies in each project
// directory. A depth of zero scans only dir.
func DetectProjectWithDepth(dir string, plugins []*plugin.Plugin, depthLimit int) []*DetectedProject {
	var detected []*DetectedProject

	// Check current directory
	detected = append(detected, detectInDir(dir, plugins)...)

	// Check subdirectories up to maxDepth
	var walk func(currentDir string, depth int)
	walk = func(currentDir string, depth int) {
		if depth >= depthLimit {
			return
		}

		entries, err := os.ReadDir(currentDir)
		if err != nil {
			return
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			// Skip hidden directories and common non-project dirs
			name := entry.Name()
			if len(name) == 0 || name[0] == '.' || name == "node_modules" || name == "vendor" || name == "target" {
				continue
			}

			subDir := filepath.Join(currentDir, name)
			subDetected := detectInDir(subDir, plugins)
			detected = append(detected, subDetected...)

			walk(subDir, depth+1)
		}
	}

	walk(dir, 0)
	detected = groupByDirectory(detected)

	// Sort by confidence descending
	sort.Slice(detected, func(i, j int) bool {
		return detected[i].Confidence > detected[j].Confidence
	})

	return detected
}

func detectInDir(dir string, plugins []*plugin.Plugin) []*DetectedProject {
	var detected []*DetectedProject

	for _, p := range plugins {
		score := 0

		for _, f := range p.Detect.Files {
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				score += 10
			}
		}

		for _, d := range p.Detect.Dirs {
			if info, err := os.Stat(filepath.Join(dir, d)); err == nil && info.IsDir() {
				score += 5
			}
		}

		if score > 0 {
			resolved := plugin.ResolveForDir(p, dir)
			detected = append(detected, &DetectedProject{
				Plugin:     resolved,
				Types:      []string{resolved.Name},
				Confidence: score,
				Dir:        dir,
			})
		}
	}

	return detected
}

func groupByDirectory(detected []*DetectedProject) []*DetectedProject {
	byDir := make(map[string]*DetectedProject)
	order := make([]string, 0)
	for _, candidate := range detected {
		group, ok := byDir[candidate.Dir]
		if !ok {
			copyPlugin := *candidate.Plugin
			copyPlugin.Commands = append([]plugin.Command(nil), candidate.Plugin.Commands...)
			copyPlugin.Env = append([]plugin.EnvCheck(nil), candidate.Plugin.Env...)
			group = &DetectedProject{Plugin: &copyPlugin, Types: append([]string(nil), candidate.Types...), Confidence: candidate.Confidence, Dir: candidate.Dir}
			byDir[candidate.Dir] = group
			order = append(order, candidate.Dir)
			continue
		}
		if len(group.Types) == 1 {
			for i := range group.Plugin.Commands {
				group.Plugin.Commands[i].Name = group.Types[0] + ": " + group.Plugin.Commands[i].Name
			}
		}
		group.Types = append(group.Types, candidate.Types...)
		group.Confidence += candidate.Confidence
		for _, command := range candidate.Plugin.Commands {
			command.Name = candidate.Plugin.Name + ": " + command.Name
			group.Plugin.Commands = append(group.Plugin.Commands, command)
		}
		group.Plugin.Env = appendUniqueEnv(group.Plugin.Env, candidate.Plugin.Env)
		group.Plugin.Name = strings.Join(group.Types, ", ")
		group.Plugin.Version = "multiple"
	}
	result := make([]*DetectedProject, 0, len(order))
	for _, dir := range order {
		result = append(result, byDir[dir])
	}
	return result
}

func appendUniqueEnv(current, additional []plugin.EnvCheck) []plugin.EnvCheck {
	seen := make(map[string]bool, len(current))
	for _, check := range current {
		seen[check.Name] = true
	}
	for _, check := range additional {
		if !seen[check.Name] {
			current = append(current, check)
			seen[check.Name] = true
		}
	}
	return current
}
