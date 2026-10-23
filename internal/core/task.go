package core

import (
	"strings"

	"github.com/cyamoyed/proton/internal/envcheck"
)

// TaskNeedsApproval 报告任务是否被标记为高危或显式要求确认，
// 非交互执行这类任务前必须获得明确批准（CLI 的 --yes / MCP 的 approved）。
func TaskNeedsApproval(task Task) bool {
	return task.Confirm || strings.EqualFold(task.Risk, "high")
}

// MissingRequiredTools 返回任务的必需工具中未通过环境检查的名称。
func MissingRequiredTools(task Task) []string {
	var missing []string
	for _, status := range envcheck.CheckEnvironment(task.Checks) {
		if status.Required && !status.OK {
			missing = append(missing, status.Name)
		}
	}
	return missing
}
