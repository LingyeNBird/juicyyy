package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Background(lipgloss.Color("236")).Padding(0, 1)
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	goodStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	badStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	paneStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1)
	inputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230"))
)

func (m appModel) View() string {
	if m.mode == addMode {
		return m.formView()
	}
	return m.listView()
}

func (m appModel) listView() string {
	header := titleStyle.Render(m.tr("Juicy 批量检测器", "Juicy Batch Checker")) + "  " + mutedStyle.Render(m.tr("提示词：", "Prompt: ")+juicyPrompt)
	providerPane := paneStyle.Width(maxInt(36, m.width/2-3)).Render(m.providerListView())
	resultPane := paneStyle.Width(maxInt(36, m.width/2-3)).Render(m.resultListView())
	body := lipgloss.JoinHorizontal(lipgloss.Top, providerPane, resultPane)
	footer := helpStyle.Render(m.tr("快捷键：a 新增供应商 | Enter 开始检测 | j/k 移动 | l 切换中英 | q 退出", "Keys: a add provider | enter run checks | j/k move | l toggle lang | q quit"))

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		body,
		"",
		m.statusLine(),
		footer,
	)
}

func (m appModel) formView() string {
	lines := []string{
		titleStyle.Render(m.tr("新增供应商", "Add Provider")),
		"",
		m.tr("请填写 OAI 兼容 base URL、API key 和模型列表（逗号分隔）。", "Fill in an OAI-compatible base URL, API key, and comma-separated models."),
		"",
		m.tr("基础 URL", "Base URL"),
		m.inputs[0].View(),
		"",
		m.tr("API 密钥", "API Key"),
		m.inputs[1].View(),
		"",
		m.tr("模型列表", "Models"),
		m.inputs[2].View(),
	}

	formPane := paneStyle.Width(maxInt(72, m.width-6)).Render(strings.Join(lines, "\n"))
	footer := helpStyle.Render(m.tr("快捷键：tab/shift+tab 切换焦点 | Enter 保存 | Esc 取消 | l 切换中英", "Keys: tab/shift+tab move | enter save | esc cancel | l toggle lang"))

	return lipgloss.JoinVertical(lipgloss.Left,
		formPane,
		"",
		m.statusLine(),
		footer,
	)
}

func (m appModel) providerListView() string {
	if len(m.config.Providers) == 0 {
		return m.tr("供应商\n\n还没有保存任何供应商，按 'a' 新增。", "Providers\n\nNo providers saved yet. Press 'a' to add one.")
	}

	lines := []string{m.tr("供应商", "Providers"), ""}
	for i, provider := range m.config.Providers {
		cursor := "  "
		line := fmt.Sprintf(m.tr("%s（%d 个模型）", "%s (%d models)"), provider.BaseURL, len(provider.Models))
		if i == m.cursor {
			cursor = "> "
			line = goodStyle.Render(line)
		}
		lines = append(lines, cursor+line)
		lines = append(lines, mutedStyle.Render("   "+strings.Join(provider.Models, ", ")))
	}

	return strings.Join(lines, "\n")
}

func (m appModel) resultListView() string {
	lines := []string{m.tr("结果", "Results"), ""}
	if m.running {
		lines = append(lines, m.spinner.View()+" "+m.tr("正在执行检测...", "Running juicy checks..."))
	}
	if len(m.results) == 0 {
		if !m.running {
			lines = append(lines, m.tr("暂无结果，请先选择供应商并按 Enter。", "No results yet. Select a provider and press Enter."))
		}
		return strings.Join(lines, "\n")
	}

	for _, result := range m.results {
		if result.Error != "" {
			lines = append(lines, badStyle.Render(fmt.Sprintf("%s -> %s", result.Model, result.Error)))
			continue
		}
		lines = append(lines, goodStyle.Render(fmt.Sprintf("%s -> %s", result.Model, result.Value)))
	}

	return strings.Join(lines, "\n")
}

func (m appModel) statusLine() string {
	if strings.TrimSpace(m.status) == "" {
		return mutedStyle.Render(m.tr("就绪", "Ready"))
	}
	return mutedStyle.Render(m.status)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
