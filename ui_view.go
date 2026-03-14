package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	pageTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("153")).Padding(0, 1)
	fieldLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))
	helperTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	mutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	selectionStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("153"))
	loadingStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	successStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	infoStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
	warningStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
	paneBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("177"))
	paneStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("177")).Padding(1)
	inputStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

func (m appModel) View() string {
	if m.mode == addMode {
		return m.formView()
	}
	return m.listView()
}

func (m appModel) listView() string {
	header := m.renderPageHeader(
		m.tr("Juicy 批量检测器", "Juicy Batch Checker"),
		m.tr("提示词：", "Prompt: ")+juicyPrompt,
	)
	bottomContent := m.listBottomContent()
	paneWidth := listPaneWidth(m.width)
	bodyHeight := m.availableListBodyHeight(header, bottomContent)
	providerPane := renderTitledPaneWithHeight(
		m.tr("供应商", "Providers"),
		paneWidth,
		bodyHeight,
		m.providerListView(),
	)
	resultPane := renderTitledPaneWithHeight(
		m.tr("结果", "Results"),
		paneWidth,
		bodyHeight,
		m.resultListView(),
	)
	body := lipgloss.JoinHorizontal(lipgloss.Top, providerPane, resultPane)
	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		body,
	)
	view := m.renderViewWithBottomBar(mainContent, bottomContent)
	m.logListLayout(
		lipgloss.Height(header),
		lipgloss.Height(bottomContent),
		bodyHeight,
		lipgloss.Height(providerPane),
		lipgloss.Height(resultPane),
		lipgloss.Height(body),
		lipgloss.Height(mainContent),
		lipgloss.Height(view),
	)

	return view
}

func (m appModel) listBottomContent() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.statusLine(),
		renderShortcutFooter(m.tr("快捷键：a 新增供应商 | Enter 开始检测 | j/k 移动 | l 切换中英 | q 退出", "Keys: a add provider | enter run checks | j/k move | l toggle lang | q quit")),
	)
}

func (m appModel) formView() string {
	paneWidth := formPaneWidth(m.width)
	applyInputLocale(m.inputs, m.lang, paneWidth)

	sections := []string{
		helperTextStyle.Render(m.tr("请填写 OAI 兼容 base URL、API key 和模型列表（逗号分隔）。", "Fill in an OAI-compatible base URL, API key, and comma-separated models.")),
	}
	for i, field := range formFields {
		sections = append(sections, m.renderFormField(field, m.inputs[i].View()))
	}

	formPane := renderTitledPane(m.tr("新增供应商", "Add Provider"), paneWidth, strings.Join(sections, "\n\n"))
	bottomContent := lipgloss.JoinVertical(lipgloss.Left,
		m.statusLine(),
		renderShortcutFooter(m.tr("快捷键：tab/shift+tab 切换焦点 | Enter 保存 | Esc 取消 | l 切换中英", "Keys: tab/shift+tab move | enter save | esc cancel | l toggle lang")),
	)
	view := m.renderViewWithBottomBar(formPane, bottomContent)
	m.logFormLayout(
		paneWidth,
		lipgloss.Height(formPane),
		lipgloss.Height(bottomContent),
		lipgloss.Height(formPane),
		lipgloss.Height(view),
	)

	return view
}

func (m appModel) renderViewWithBottomBar(mainContent, bottomContent string) string {
	if m.height <= 0 {
		return lipgloss.JoinVertical(lipgloss.Left,
			mainContent,
			bottomContent,
		)
	}

	remainingHeight := m.height - lipgloss.Height(mainContent) - lipgloss.Height(bottomContent)
	if remainingHeight > 0 {
		spacer := lipgloss.NewStyle().Height(remainingHeight).Render("")
		return lipgloss.JoinVertical(lipgloss.Left,
			mainContent,
			spacer,
			bottomContent,
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		mainContent,
		bottomContent,
	)
}

func (m appModel) providerListView() string {
	lines := []string{}
	if len(m.config.Providers) == 0 {
		lines = append(lines, renderEmptyState(m.tr("还没有保存任何供应商，按 'a' 新增。", "No providers saved yet. Press 'a' to add one.")))
		return strings.Join(lines, "\n")
	}

	for i, provider := range m.config.Providers {
		cursor := "  "
		line := fmt.Sprintf(m.tr("%s（%d 个模型）", "%s (%d models)"), provider.BaseURL, len(provider.Models))
		if i == m.cursor {
			cursor = selectionStyle.Render("> ")
			line = selectionStyle.Render(line)
		}
		lines = append(lines, cursor+line)
		lines = append(lines, helperTextStyle.Render("   "+strings.Join(provider.Models, ", ")))
	}

	return strings.Join(lines, "\n")
}

func (m appModel) resultListView() string {
	lines := []string{}
	if m.running {
		lines = append(lines, loadingStyle.Render(m.spinner.View()+" "+m.tr("正在执行检测...", "Running juicy checks...")))
	}
	if len(m.results) == 0 {
		if !m.running {
			lines = append(lines, renderEmptyState(m.tr("暂无结果，请先选择供应商并按 Enter。", "No results yet. Select a provider and press Enter.")))
		}
		return strings.Join(lines, "\n")
	}

	for _, result := range m.results {
		if result.Error != "" {
			lines = append(lines, errorStyle.Render(fmt.Sprintf("%s -> %s", result.Model, result.Error)))
			continue
		}
		lines = append(lines, successStyle.Render(fmt.Sprintf("%s -> %s", result.Model, result.Value)))
	}

	return strings.Join(lines, "\n")
}

func (m appModel) statusLine() string {
	text := strings.TrimSpace(m.status)
	if text == "" {
		text = m.tr("就绪", "Ready")
	}

	style := infoStyle
	switch m.statusKind {
	case statusSuccess:
		style = successStyle
	case statusError:
		style = errorStyle
	case statusWarning:
		style = warningStyle
	case statusLoading:
		style = loadingStyle
	}
	return style.Render(text)
}

func (m appModel) renderPageHeader(title, subtitle string) string {
	lines := []string{pageTitleStyle.Render(title)}
	if strings.TrimSpace(subtitle) != "" {
		lines = append(lines, helperTextStyle.Render(subtitle))
	}
	return strings.Join(lines, "\n")
}

func (m appModel) renderFormField(field formFieldSpec, inputView string) string {
	return strings.Join([]string{
		renderFieldLabel(field.label.forLang(m.lang)),
		inputView,
		helperTextStyle.Render(field.helper.forLang(m.lang)),
	}, "\n")
}

func renderPaneTitle(title string) string {
	return paneBorderStyle.Copy().Bold(true).Render("|" + title + "|")
}

func renderTitledPane(title string, width int, body string) string {
	rendered := paneStyle.Width(width).Render(body)
	return renderTitledPaneFromRendered(title, rendered)
}

func renderTitledPaneFromRendered(title, rendered string) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	border := lipgloss.RoundedBorder()
	titleText := renderPaneTitle(title)
	visibleTitleWidth := lipgloss.Width("|" + title + "|")
	interiorWidth := maxInt(0, lipgloss.Width(lines[0])-2)
	leftRun := 1
	rightRun := interiorWidth - leftRun - visibleTitleWidth
	if rightRun < 0 {
		rightRun = 0
		leftRun = maxInt(0, interiorWidth-visibleTitleWidth)
	}

	lines[0] = strings.Join([]string{
		paneBorderStyle.Render(border.TopLeft + strings.Repeat(border.Top, leftRun)),
		titleText,
		paneBorderStyle.Render(strings.Repeat(border.Top, rightRun) + border.TopRight),
	}, "")

	return strings.Join(lines, "\n")
}

func renderTitledPaneWithHeight(title string, width, height int, body string) string {
	if height <= 0 {
		return renderTitledPane(title, width, body)
	}

	contentHeight := maxInt(0, height-paneVerticalChrome)
	wrappedBody := wrapPaneBody(width, body)
	lines := []string{}
	if wrappedBody != "" {
		lines = strings.Split(wrappedBody, "\n")
	}

	switch {
	case len(lines) > contentHeight:
		lines = lines[:contentHeight]
	case len(lines) < contentHeight:
		lines = append(lines, make([]string, contentHeight-len(lines))...)
	}

	rendered := paneStyle.Width(width).Render(strings.Join(lines, "\n"))
	return renderTitledPaneFromRendered(title, rendered)
}

func wrapPaneBody(width int, body string) string {
	if body == "" {
		return ""
	}

	contentWidth := maxInt(0, width-paneStyle.GetPaddingLeft()-paneStyle.GetPaddingRight())
	if contentWidth == 0 {
		return body
	}

	return lipgloss.NewStyle().Width(contentWidth).Render(body)
}

func renderFieldLabel(label string) string {
	return fieldLabelStyle.Render(label)
}

func renderEmptyState(text string) string {
	return helperTextStyle.Render(text)
}

func renderShortcutFooter(text string) string {
	return helperTextStyle.Render(text)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
