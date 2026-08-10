package tui

import "github.com/charmbracelet/lipgloss"

// 配色
const (
	colorPurple = "#7D56F4" // 品牌主色
	colorBorder = "#6B5BA8" // 面板边框
	colorWhite  = "#FAFAFA"
	colorGreen  = "#04B575"
	colorRed    = "#FF6B6B"
	colorAmber  = "#FFB454"
	colorDim    = "#8B8B93"
)

var (
	// 标题栏
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorWhite)).
			Background(lipgloss.Color(colorPurple)).
			Padding(0, 2)

	// 面板容器
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorder)).
			Padding(0, 1)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorPurple))

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBorder))

	// 标签与选中项
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorPurple))

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorPurple))

	cursorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorPurple))

	// 弱化文本
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim))

	// 环境检查
	envOKStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorGreen))

	envFailStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorRed))

	requiredTagStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorRed))

	checkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAmber))

	// 风险徽标
	riskHighStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorRed))

	riskMidStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorAmber))

	riskLowStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorDim))

	// 提示与确认
	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim))

	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorAmber))

	confirmBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorAmber)).
			Padding(0, 2)

	// 底部快捷键
	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorPurple))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim))
)
