package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/fupeijun/proton/internal/detector"
	"github.com/fupeijun/proton/internal/envcheck"
	"github.com/fupeijun/proton/internal/plugin"
)

type panel int

const (
	commandsPanel panel = iota
	envPanel
)

// envCheckResultMsg is sent when environment check completes
type envCheckResultMsg struct {
	statuses []envcheck.EnvStatus
	request  int
}

// checkEnvCmd runs environment check asynchronously
func checkEnvCmd(checks []plugin.EnvCheck, request int) tea.Cmd {
	return func() tea.Msg {
		statuses := envcheck.CheckEnvironment(checks)
		return envCheckResultMsg{statuses: statuses, request: request}
	}
}

// Model is the TUI model
type Model struct {
	projects      []*detector.DetectedProject
	current       int
	commands      []plugin.Command
	envStatus     []envcheck.EnvStatus
	envChecking   bool
	envRequest    int
	initCmd       tea.Cmd
	cursor        int
	panel         panel
	quitting      bool
	executeCmd    string
	executeDir    string
	executeOnExit bool
	confirming    bool
	pending       plugin.Command
	notice        string
}

// NewModel creates a new TUI model
func NewModel(projects []*detector.DetectedProject) Model {
	m := Model{
		projects: projects,
		current:  0,
		panel:    commandsPanel,
	}

	if len(projects) > 0 {
		m.commands = projects[0].Plugin.Commands
		m.envChecking = true
		m.envRequest = 1
		m.initCmd = checkEnvCmd(projects[0].Plugin.Env, m.envRequest)
	}

	return m
}

// Init initializes the TUI model
func (m Model) Init() tea.Cmd {
	return m.initCmd
}

// Update handles TUI updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case envCheckResultMsg:
		if msg.request != m.envRequest {
			return m, nil
		}
		m.envStatus = msg.statuses
		m.envChecking = false
		return m, nil

	case tea.KeyMsg:
		if m.confirming {
			switch msg.String() {
			case "y", "Y", "enter":
				m.executeCmd = m.pending.Run
				m.executeDir = m.projects[m.current].Dir
				m.executeOnExit = true
				m.quitting = true
				return m, tea.Quit
			case "n", "N", "esc", "q":
				m.confirming = false
				m.pending = plugin.Command{}
				m.notice = "Execution cancelled"
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "tab":
			if m.panel == commandsPanel {
				m.panel = envPanel
			} else {
				m.panel = commandsPanel
			}
			m.cursor = 0

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			maxCursor := 0
			if m.panel == commandsPanel {
				maxCursor = len(m.commands) - 1
			} else {
				maxCursor = len(m.envStatus) - 1
			}
			if m.cursor < maxCursor {
				m.cursor++
			}

		case "enter":
			if m.panel == commandsPanel && m.cursor < len(m.commands) {
				selected := m.commands[m.cursor]
				cmd := selected.Run
				if cmd == "" {
					// Ignore empty commands
					return m, nil
				}
				if m.envChecking {
					m.notice = "Wait for the environment check to finish"
					return m, nil
				}
				if missing := missingRequired(m.envStatus); len(missing) > 0 {
					m.notice = "Cannot execute: missing required tools: " + fmt.Sprintf("%v", missing)
					return m, nil
				}
				if selected.Confirm || selected.Risk == "high" {
					m.confirming = true
					m.pending = selected
					m.notice = ""
					return m, nil
				}
				// Store command to execute after TUI exits
				m.executeCmd = cmd
				m.executeDir = m.projects[m.current].Dir
				m.executeOnExit = true
				m.quitting = true
				return m, tea.Quit
			}

		case "left":
			if m.current > 0 {
				m.current--
				m.commands = m.projects[m.current].Plugin.Commands
				m.envChecking = true
				m.envStatus = nil
				m.envRequest++
				m.cursor = 0
				m.notice = ""
				return m, checkEnvCmd(m.projects[m.current].Plugin.Env, m.envRequest)
			}

		case "right":
			if m.current < len(m.projects)-1 {
				m.current++
				m.commands = m.projects[m.current].Plugin.Commands
				m.envChecking = true
				m.envStatus = nil
				m.envRequest++
				m.cursor = 0
				m.notice = ""
				return m, checkEnvCmd(m.projects[m.current].Plugin.Env, m.envRequest)
			}
		}
	}
	return m, nil
}

// View renders the TUI
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if len(m.projects) == 0 {
		return "No projects detected in current directory.\n"
	}

	project := m.projects[m.current]

	// Title
	title := titleStyle.Render("Proton - Project Launcher")

	// Project info
	info := fmt.Sprintf("%s %s\n%s %s\n%s %s",
		headerStyle.Render("Project:"),
		filepathBase(project.Dir),
		headerStyle.Render("Type:"),
		strings.Join(project.Types, ", "),
		headerStyle.Render("Location:"),
		project.Dir,
	)

	// Environment panel
	envView := headerStyle.Render("Environment") + "\n"
	if m.envChecking {
		envView += "  Checking environment...\n"
	} else {
		for i, status := range m.envStatus {
			prefix := "  "
			if i == m.cursor && m.panel == envPanel {
				prefix = "> "
			}

			if status.OK {
				envView += envOKStyle.Render(fmt.Sprintf("%s✓ %s %s", prefix, status.Name, status.Version)) + "\n"
			} else {
				envView += envFailStyle.Render(fmt.Sprintf("%s✗ %s (not installed)", prefix, status.Name)) + "\n"
			}
		}
	}

	// Commands panel
	cmdView := headerStyle.Render("Commands") + "                    " + helpStyle.Render("[Tab] to switch") + "\n"
	for i, cmd := range m.commands {
		prefix := " "
		if i == m.cursor && m.panel == commandsPanel {
			prefix = ">"
		}
		risk := ""
		if cmd.Risk != "" {
			risk = " [" + cmd.Risk + "]"
		}
		cmdView += fmt.Sprintf("%s %s%s  %s\n", prefix, cmd.Name, risk, helpStyle.Render(cmd.Run))
	}
	if m.panel == commandsPanel && m.cursor < len(m.commands) && m.commands[m.cursor].Description != "" {
		cmdView += "\n" + helpStyle.Render(m.commands[m.cursor].Description) + "\n"
	}

	// Help
	help := helpStyle.Render("[Enter] Execute   [←/→] Switch Project   [q] Quit")
	if m.confirming {
		help = warningStyle.Render(fmt.Sprintf("Run %q? [y/Enter] Confirm  [n/Esc] Cancel", m.pending.Run))
	} else if m.notice != "" {
		help = warningStyle.Render(m.notice)
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s",
		title, info, envView, cmdView, help)
}

func missingRequired(statuses []envcheck.EnvStatus) []string {
	var missing []string
	for _, status := range statuses {
		if status.Required && !status.OK {
			missing = append(missing, status.Name)
		}
	}
	return missing
}

func filepathBase(path string) string {
	for len(path) > 1 && (path[len(path)-1] == '/' || path[len(path)-1] == '\\') {
		path = path[:len(path)-1]
	}
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}

// GetExecuteCmd returns the command to execute after TUI exits
func (m Model) GetExecuteCmd() (command, dir string, ok bool) {
	return m.executeCmd, m.executeDir, m.executeOnExit
}
