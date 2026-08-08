package detector

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cyamoyed/proton/internal/plugin"
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
			if shouldSkipDirectory(name) {
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
		var matchedFiles []os.FileInfo
		var matchedDirs []os.FileInfo

		for _, f := range p.Detect.Files {
			for _, match := range detectionMatches(dir, f, false) {
				if appendUniqueFileInfo(&matchedFiles, match) {
					score += 10
				}
			}
		}

		for _, d := range p.Detect.Dirs {
			for _, match := range detectionMatches(dir, d, true) {
				if appendUniqueFileInfo(&matchedDirs, match) {
					score += 5
				}
			}
		}

		if score > 0 && plugin.MatchesDir(p, dir) {
			resolved := plugin.ResolveForDir(p, dir)
			for i := range resolved.Commands {
				if resolved.Commands[i].Confidence == 0 {
					resolved.Commands[i].Confidence = score
				}
			}
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

func shouldSkipDirectory(name string) bool {
	if len(name) == 0 || name[0] == '.' {
		return true
	}
	switch name {
	case "node_modules", "vendor", "target", "Pods", "Carthage", "DerivedData", ".build":
		return true
	}
	return strings.HasSuffix(name, ".xcodeproj") || strings.HasSuffix(name, ".xcworkspace")
}

func detectionMatches(dir, pattern string, wantDirectory bool) []os.FileInfo {
	paths := []string{filepath.Join(dir, pattern)}
	if strings.ContainsAny(pattern, "*?[") {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil
		}
		paths = matches
	}
	var matches []os.FileInfo
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() == wantDirectory {
			matches = append(matches, info)
		}
	}
	return matches
}

func appendUniqueFileInfo(current *[]os.FileInfo, candidate os.FileInfo) bool {
	for _, existing := range *current {
		if os.SameFile(existing, candidate) {
			return false
		}
	}
	*current = append(*current, candidate)
	return true
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
