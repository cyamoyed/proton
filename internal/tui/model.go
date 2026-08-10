package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyamoyed/proton/internal/detector"
	"github.com/cyamoyed/proton/internal/envcheck"
	"github.com/cyamoyed/proton/internal/plugin"
	"github.com/mattn/go-runewidth"
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
	width         int
}

// NewModel creates a new TUI model
func NewModel(projects []*detector.DetectedProject) Model {
	m := Model{
		projects: projects,
		current:  0,
		panel:    commandsPanel,
		width:    80,
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

	case tea.WindowSizeMsg:
		m.width = msg.Width

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
				m.notice = "已取消执行"
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
					m.notice = "请等待环境检查完成"
					return m, nil
				}
				if missing := missingRequiredForCommand(m.envStatus, selected.Checks); len(missing) > 0 {
					m.notice = "无法执行：缺少必需工具：" + strings.Join(missing, "、")
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
		return "\n  " + infoStyle.Render("未检测到项目，请确认当前目录包含受支持的项目文件。") + "\n"
	}

	width := m.width
	if width < 40 {
		width = 80 // 终端尺寸未知时使用默认宽度
	}
	contentWidth := width - 4 // 边框(2) + 内边距(2)

	project := m.projects[m.current]

	var sections []string

	// 标题栏
	sections = append(sections,
		lipgloss.PlaceHorizontal(width, lipgloss.Center, titleStyle.Render(" Proton 项目启动器 ")))

	// 项目信息面板
	projectTitle := "项目"
	if len(m.projects) > 1 {
		projectTitle = fmt.Sprintf("项目 %d/%d", m.current+1, len(m.projects))
	}
	var infoLines []string
	infoLines = append(infoLines, labelLine("名称", selectedStyle.Render(filepathBase(project.Dir))))
	infoLines = append(infoLines, labelLine("类型", strings.Join(project.Types, ", ")))
	infoLines = append(infoLines, labelLine("位置", dimStyle.Render(truncate(project.Dir, contentWidth-6))))
	sections = append(sections, renderPanel(projectTitle, infoLines, contentWidth))

	// 命令面板
	var cmdLines []string
	if len(m.commands) == 0 {
		cmdLines = append(cmdLines, infoStyle.Render("无可用命令"))
	} else {
		for i, cmd := range m.commands {
			row, _ := commandRow(cmd, m.panel == commandsPanel && i == m.cursor, contentWidth)
			cmdLines = append(cmdLines, row)
		}
		if m.panel == commandsPanel && m.cursor < len(m.commands) {
			selected := m.commands[m.cursor]
			var detail []string
			if selected.Description != "" {
				detail = append(detail, wrapText(selected.Description, contentWidth-2))
			}
			if selected.Source != "" {
				detail = append(detail, dimStyle.Render("来源："+truncate(selected.Source, contentWidth-6)))
			}
			if _, truncated := commandRow(selected, true, contentWidth); truncated {
				// 行内放不下时，在详情区显示完整命令
				detail = append(detail, dimStyle.Render("$ "+wrapText(selected.Run, contentWidth-4)))
			}
			if len(detail) > 0 {
				cmdLines = append(cmdLines, append([]string{""}, detail...)...)
			}
		}
	}
	sections = append(sections, renderPanel("命令", cmdLines, contentWidth))

	// 环境面板
	var envLines []string
	switch {
	case m.envChecking:
		envLines = append(envLines, checkingStyle.Render("⟳ 正在检查环境…"))
	case len(m.envStatus) == 0:
		envLines = append(envLines, infoStyle.Render("无环境检查"))
	default:
		for i, status := range m.envStatus {
			envLines = append(envLines, envRow(status, m.panel == envPanel && i == m.cursor, contentWidth))
		}
	}
	sections = append(sections, renderPanel("环境", envLines, contentWidth))

	// 底部提示
	var footer string
	switch {
	case m.confirming:
		footer = renderConfirm(m.pending, contentWidth)
	case m.notice != "":
		footer = warningStyle.Render("⚠ " + m.notice)
	default:
		footer = renderHelp()
	}
	sections = append(sections, footer)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderPanel 渲染带标题和边框的面板
func renderPanel(title string, lines []string, contentWidth int) string {
	var b strings.Builder
	b.WriteString(panelTitleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", contentWidth)))
	for _, line := range lines {
		b.WriteString("\n")
		b.WriteString(padRight(line, contentWidth))
	}
	return panelStyle.Render(b.String())
}

// padRight 用空格补齐到指定显示宽度
func padRight(s string, w int) string {
	if w <= 0 {
		return s
	}
	if width := lipgloss.Width(s); width < w {
		return s + strings.Repeat(" ", w-width)
	}
	return s
}

// labelLine 渲染「标签 值」行，标签列宽固定为 6
func labelLine(label, value string) string {
	return headerStyle.Render(label) + strings.Repeat(" ", 6-lipgloss.Width(label)) + value
}

// wrapText 按显示宽度自动换行
func wrapText(s string, w int) string {
	if w < 1 {
		return s
	}
	return lipgloss.NewStyle().Width(w).Render(s)
}

// truncate 按显示宽度截断文本，超出部分以省略号代替
func truncate(s string, max int) string {
	if max <= 0 || lipgloss.Width(s) <= max {
		return s
	}
	runes := []rune(s)
	used := 0
	cut := 0
	for i, r := range runes {
		width := runewidth.RuneWidth(r)
		if used+width > max-1 {
			cut = i
			break
		}
		used += width
		cut = i + 1
	}
	return string(runes[:cut]) + "…"
}

// commandRow 渲染单条命令行：名称 + 原命令 + 风险徽标。
// 返回是否因宽度不足截断了原命令。
func commandRow(cmd plugin.Command, selected bool, contentWidth int) (string, bool) {
	mark := "  "
	name := cmd.Name
	if selected {
		mark = cursorStyle.Render("▶ ")
		name = selectedStyle.Render(name)
	}
	line := mark + name
	badge := ""
	if b, ok := riskBadge(cmd.Risk); ok {
		badge = b
	}
	truncated := false
	if cmd.Run != "" {
		run := "$ " + cmd.Run
		runMax := contentWidth - 2 - lipgloss.Width(line) - lipgloss.Width(badge) - 3
		if runMax < 8 {
			runMax = 8
		}
		shown := truncate(run, runMax)
		if shown != run {
			truncated = true
		}
		line += dimStyle.Render("  " + shown)
	}
	if badge != "" {
		pad := contentWidth - 2 - lipgloss.Width(line) - lipgloss.Width(badge)
		if pad > 1 {
			line += strings.Repeat(" ", pad) + badge
		} else {
			line += " " + badge
		}
	}
	return line, truncated
}

// riskBadge 将风险等级映射为中文徽标
func riskBadge(risk string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(risk)) {
	case "", "-", "none":
		return "", false
	case "high":
		return riskHighStyle.Render("[高危]"), true
	case "medium":
		return riskMidStyle.Render("[中危]"), true
	case "low":
		return riskLowStyle.Render("[低危]"), true
	default:
		return riskMidStyle.Render("[" + risk + "]"), true
	}
}

// envRow 渲染单条环境检查结果
func envRow(status envcheck.EnvStatus, selected bool, contentWidth int) string {
	mark := "  "
	if selected {
		mark = cursorStyle.Render("▶ ")
	}
	if status.OK {
		line := mark + envOKStyle.Render("✓ "+status.Name)
		if status.Version != "" {
			pad := 14 - lipgloss.Width(status.Name)
			if pad < 1 {
				pad = 1
			}
			versionWidth := contentWidth - 18
			if versionWidth < 10 {
				versionWidth = 10
			}
			line += strings.Repeat(" ", pad) + dimStyle.Render(truncate(status.Version, versionWidth))
		}
		return line
	}
	line := mark + envFailStyle.Render("✗ "+status.Name)
	if status.Required {
		line += " " + requiredTagStyle.Render("必需")
	} else {
		line += " " + dimStyle.Render("未安装")
	}
	return line
}

// renderHelp 渲染底部快捷键提示
func renderHelp() string {
	key := func(label string) string { return keyStyle.Render(label) }
	return footerStyle.Render(fmt.Sprintf(
		"%s 执行命令    %s 切换面板    %s 切换项目    %s 退出",
		key("Enter"), key("Tab"), key("← →"), key("q"),
	))
}

// renderConfirm 渲染确认执行提示
func renderConfirm(pending plugin.Command, contentWidth int) string {
	command := truncate(pending.Run, max(contentWidth-6, 20))
	key := func(label string) string { return keyStyle.Render(label) }
	content := fmt.Sprintf("确认执行以下命令？\n\n    %s\n\n  %s 确认    %s 取消",
		command, key("y / Enter"), key("n / Esc"))
	return confirmBoxStyle.Width(contentWidth + 2).Render(content)
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

func missingRequiredForCommand(statuses []envcheck.EnvStatus, checks []plugin.EnvCheck) []string {
	if checks == nil {
		return missingRequired(statuses)
	}
	required := make(map[string]bool)
	for _, check := range checks {
		if check.Required {
			required[check.Name] = true
		}
	}
	available := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		available[status.Name] = status.OK
	}
	var missing []string
	for _, check := range checks {
		if required[check.Name] && !available[check.Name] {
			missing = append(missing, check.Name)
			delete(required, check.Name)
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
