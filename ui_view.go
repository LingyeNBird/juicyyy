package main

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

var (
	trailingANSIPattern        = regexp.MustCompile(`(?:\x1b\[[0-9;?]*[ -/]*[@-~])*$`)
	inactivePaneBorderColor    = lipgloss.Color("242")
	defaultPaneBorderColor     = lipgloss.Color("177")
	resultsPaneBorderColor     = lipgloss.Color("214")
	addProviderPaneBorderColor = lipgloss.Color("78")
	requestPaneBorderColor     = lipgloss.Color("203")

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
	paneBorderStyle = lipgloss.NewStyle().Foreground(defaultPaneBorderColor)
	paneStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(defaultPaneBorderColor).Padding(1)
	inputStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

type paneScrollbarMeta struct {
	totalLines     int
	visibleLines   int
	viewportOffset int
}

type splitPane struct {
	title          string
	body           string
	viewportOffset int
	borderColor    lipgloss.Color
}

func (m appModel) View() string {
	if m.mode != listMode {
		return m.formView()
	}
	return m.listView()
}

func (m appModel) listView() string {
	providerPane := splitPane{
		title:          m.listProviderPaneTitle(),
		body:           m.providerPaneLayout().body,
		viewportOffset: m.providerPaneScrollOffset,
		borderColor:    m.listPaneBorderColor(providerPaneFocus),
	}
	resultsPane := splitPane{
		title:          m.listResultsPaneTitle(),
		body:           m.resultPaneLayout().body,
		viewportOffset: m.resultsPaneScrollOffset,
		borderColor:    m.listPaneBorderColor(resultsPaneFocus),
	}

	return m.renderSplitView(
		providerPane,
		resultsPane,
		m.listBottomContent(),
	)
}

func (m appModel) renderSplitView(leftPane, rightPane splitPane, bottomContent string) string {
	header := m.pageHeader()
	paneWidth := listPaneWidth(m.width)
	bodyHeight := m.availableListBodyHeight(header, bottomContent)
	providerPane := renderScrollableTitledPaneWithHeight(
		leftPane.title,
		paneWidth,
		bodyHeight,
		leftPane.body,
		leftPane.viewportOffset,
		leftPane.borderColor,
	)
	rightRenderedPane := renderScrollableTitledPaneWithHeight(
		rightPane.title,
		paneWidth,
		bodyHeight,
		rightPane.body,
		rightPane.viewportOffset,
		rightPane.borderColor,
	)
	body := lipgloss.JoinHorizontal(lipgloss.Top, providerPane, rightRenderedPane)
	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		body,
	)

	return m.renderViewWithBottomBar(mainContent, bottomContent)
}

func (m appModel) listBottomContent() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.statusLine(),
		renderShortcutFooter(m.tr("快捷键：p 聚焦供应商栏 | r 聚焦结果栏 | j/k/w/s/↑/↓ 切换供应商 | a 新增供应商 | e 编辑供应商 | o 请求设置 | Enter 开始检测 | l 切换中英 | q 退出", "Keys: p focus providers | r focus results | j/k/w/s/up/down move providers | a add provider | e edit provider | o request settings | enter run checks | l toggle lang | q quit")),
	)
}

func (m appModel) formView() string {
	providerPane := splitPane{
		title:          m.tr("供应商", "Providers"),
		body:           m.providerPaneLayout().body,
		viewportOffset: m.providerPaneScrollOffset,
		borderColor:    defaultPaneBorderColor,
	}
	formPane := splitPane{
		title:          m.formPaneTitle(),
		body:           m.formPaneBody(),
		viewportOffset: m.formPaneScrollOffset,
		borderColor:    m.formPaneBorderColor(),
	}

	return m.renderSplitView(
		providerPane,
		formPane,
		m.formBottomContent(),
	)
}

func (m appModel) formPaneBody() string {
	return m.formPaneLayout().body
}

func (m appModel) formPaneSections(paneWidth int) []string {
	if m.mode == requestSettingsMode {
		applyRequestSettingsLocale(&m.requestPromptInput, &m.requestTimeoutInput, &m.requestRetryInput, m.lang, paneWidth)
		return []string{
			helperTextStyle.Render(m.formIntroText()),
			m.renderFormField(requestSettingsFields[requestSettingsPromptField], m.requestPromptInput.View()),
			m.renderFormField(requestSettingsFields[requestSettingsTimeoutField], m.requestTimeoutInput.View()),
			m.renderFormField(requestSettingsFields[requestSettingsModeField], m.requestModeInputView()),
			m.renderFormField(requestSettingsFields[requestSettingsRetryField], m.requestRetryInput.View()),
		}
	}

	applyFormLocale(&m.baseURLInput, &m.apiKeyInput, &m.modelsInput, m.lang, paneWidth)
	syncModelsInputLayout(&m.modelsInput, paneWidth)

	return []string{
		helperTextStyle.Render(m.formIntroText()),
		m.renderFormField(formFields[addProviderBaseURLField], m.baseURLInput.View()),
		m.renderFormField(formFields[addProviderAPIKeyField], m.apiKeyInput.View()),
		m.renderFormField(formFields[addProviderModelsField], m.modelsInput.View()),
	}
}

func (m appModel) formPaneLayout() paneContentLayout {
	paneWidth := listPaneWidth(m.width)
	contentWidth := paneContentWidth(paneWidth)
	sections := m.formPaneSections(paneWidth)
	wrappedLines := make([]string, 0)

	for i, section := range sections {
		if i > 0 {
			wrappedLines = append(wrappedLines, "")
		}
		wrappedLines = append(wrappedLines, wrapPaneContentLines(contentWidth, section)...)
	}

	activeCursorRow := -1
	fieldCount := addProviderFieldCount
	if m.mode == requestSettingsMode {
		fieldCount = requestSettingsFieldCount
	}
	if m.focusIndex >= 0 && m.focusIndex < fieldCount && len(wrappedLines) > 0 {
		activeCursorRow = maxInt(0, minInt(m.activeFormCursorRow(paneWidth), len(wrappedLines)-1))
	}

	return paneContentLayout{
		body:            strings.Join(sections, "\n\n"),
		wrappedLines:    wrappedLines,
		activeCursorRow: activeCursorRow,
		activeEndRow:    activeCursorRow,
	}
}

func (m appModel) providerPaneLayout() paneContentLayout {
	if len(m.config.Providers) == 0 {
		body := renderEmptyState(m.tr("还没有保存任何供应商，按 'a' 新增。", "No providers saved yet. Press 'a' to add one."))
		return paneContentLayout{
			body:            body,
			wrappedLines:    wrapPaneContentLines(paneContentWidth(listPaneWidth(m.width)), body),
			activeCursorRow: -1,
			activeEndRow:    -1,
		}
	}

	paneWidth := listPaneWidth(m.width)
	contentWidth := paneContentWidth(paneWidth)
	bodyLines := make([]string, 0, len(m.config.Providers)*2)
	wrappedLines := make([]string, 0, len(m.config.Providers)*2)
	activeCursorRow := -1
	activeEndRow := -1
	row := 0

	for i, provider := range m.config.Providers {
		cursor := "  "
		line := fmt.Sprintf(m.tr("%s（%d 个模型）", "%s (%d models)"), provider.BaseURL, len(provider.Models))
		entryStartRow := row
		if i == m.cursor {
			cursor = selectionStyle.Render("> ")
			line = selectionStyle.Render(line)
		}

		entryLines := []string{
			cursor + line,
			helperTextStyle.Render("   " + strings.Join(provider.Models, ", ")),
		}
		for _, entryLine := range entryLines {
			bodyLines = append(bodyLines, entryLine)
			wrapped := wrapPaneContentLines(contentWidth, entryLine)
			wrappedLines = append(wrappedLines, wrapped...)
			row += len(wrapped)
		}
		if i == m.cursor {
			activeCursorRow = entryStartRow
			activeEndRow = maxInt(entryStartRow, row-1)
		}
	}

	return paneContentLayout{
		body:            strings.Join(bodyLines, "\n"),
		wrappedLines:    wrappedLines,
		activeCursorRow: activeCursorRow,
		activeEndRow:    activeEndRow,
	}
}

func (m appModel) resultPaneLayout() paneContentLayout {
	paneWidth := listPaneWidth(m.width)
	contentWidth := paneContentWidth(paneWidth)
	bodyLines := make([]string, 0, len(m.results)+1)
	wrappedLines := make([]string, 0, len(m.results)+1)
	activeCursorRow := -1
	activeEndRow := -1
	row := 0

	appendLine := func(line string, active bool) {
		bodyLines = append(bodyLines, line)
		lineStartRow := row
		wrapped := wrapPaneContentLines(contentWidth, line)
		if active {
			activeCursorRow = lineStartRow
			activeEndRow = maxInt(lineStartRow, lineStartRow+len(wrapped)-1)
		}
		wrappedLines = append(wrappedLines, wrapped...)
		row += len(wrapped)
	}

	if m.running {
		appendLine(loadingStyle.Render(m.spinner.View()+" "+m.tr("正在执行检测...", "Running juicy checks...")), len(m.results) == 0)
		appendLine(loadingStyle.Render(m.runProgressView(contentWidth)), false)
	}
	if len(m.results) == 0 {
		if !m.running {
			appendLine(renderEmptyState(m.tr("暂无结果，请先选择供应商并按 Enter。", "No results yet. Select a provider and press Enter.")), false)
		}
		return paneContentLayout{
			body:            strings.Join(bodyLines, "\n"),
			wrappedLines:    wrappedLines,
			activeCursorRow: activeCursorRow,
			activeEndRow:    activeEndRow,
		}
	}

	activeIndex := maxInt(0, minInt(m.activeResult, len(m.results)-1))
	for i, result := range m.results {
		if result.Error != "" {
			appendLine(errorStyle.Render(fmt.Sprintf("%s -> %s", result.Model, result.Error)), i == activeIndex)
			continue
		}
		appendLine(successStyle.Render(fmt.Sprintf("%s -> %s", result.Model, result.Value)), i == activeIndex)
	}

	return paneContentLayout{
		body:            strings.Join(bodyLines, "\n"),
		wrappedLines:    wrappedLines,
		activeCursorRow: activeCursorRow,
		activeEndRow:    activeEndRow,
	}
}

func (m appModel) runProgressView(contentWidth int) string {
	label := fmt.Sprintf("%s %d/%d", m.tr("进度", "Progress"), m.runCompleted, m.runTotal)
	barWidth := minInt(16, maxInt(0, contentWidth-lipgloss.Width(label)-3))
	if barWidth <= 0 {
		return label
	}
	return fmt.Sprintf("%s %s", label, renderASCIIProgressBar(m.runCompleted, m.runTotal, barWidth))
}

func renderASCIIProgressBar(completed, total, width int) string {
	if width <= 0 {
		return ""
	}
	filled := 0
	if total > 0 {
		filled = int(math.Round(float64(width*completed) / float64(total)))
	}
	filled = maxInt(0, minInt(filled, width))
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}

func (m appModel) formBottomContent() string {
	bottomContent := lipgloss.JoinVertical(lipgloss.Left,
		m.statusLine(),
		renderShortcutFooter(m.formFooterText()),
	)

	return bottomContent
}

func (m appModel) formPaneTitle() string {
	if m.mode == requestSettingsMode {
		return m.tr("请求设置", "Request Settings")
	}
	if m.isEditingProvider() {
		return m.tr("编辑供应商", "Edit Provider")
	}
	return m.tr("新增供应商", "Add Provider")
}

func (m appModel) listProviderPaneTitle() string {
	return m.tr("供应商[p]", "Providers[p]")
}

func (m appModel) listResultsPaneTitle() string {
	return m.tr("结果[r]", "Results[r]")
}

func (m appModel) listPaneBorderColor(focus listPaneFocus) lipgloss.Color {
	if m.listPaneFocus != focus {
		return inactivePaneBorderColor
	}
	if focus == resultsPaneFocus {
		return resultsPaneBorderColor
	}
	return defaultPaneBorderColor
}

func (m appModel) formIntroText() string {
	if m.mode == requestSettingsMode {
		return m.tr("统一管理检测请求的提示词、超时时间、接口模式和重试次数。", "Manage the prompt, timeout, API mode, and retry count used for juicy checks.")
	}
	if m.isEditingProvider() {
		return m.tr("修改当前供应商的 OAI 兼容 base URL、API key，以及支持逗号或换行的模型列表。", "Update the selected provider's OAI-compatible base URL, API key, and models separated by commas or new lines.")
	}
	return m.tr("请填写 OAI 兼容 base URL、API key，以及支持逗号或换行的模型列表。", "Fill in an OAI-compatible base URL, API key, and models separated by commas or new lines.")
}

func (m appModel) formFooterText() string {
	if m.mode == requestSettingsMode {
		return m.tr("快捷键：tab/shift+tab 切换焦点 | ←/→/Enter 切换请求方式 | Ctrl+S 保存 | Esc 取消 | Ctrl+L 切换中英", "Keys: tab/shift+tab move | left/right/enter switch request mode | ctrl+s save | esc cancel | ctrl+l toggle lang")
	}
	if m.isEditingProvider() {
		return m.tr("快捷键：tab/shift+tab 切换焦点 | Ctrl+S 更新 | 模型框 Enter 换行 | Esc 取消编辑 | Ctrl+L 切换中英", "Keys: tab/shift+tab move | ctrl+s update | enter adds a new line in Models | esc cancel edit | ctrl+l toggle lang")
	}
	return m.tr("快捷键：tab/shift+tab 切换焦点 | Ctrl+S 保存 | 模型框 Enter 换行 | Esc 取消 | Ctrl+L 切换中英", "Keys: tab/shift+tab move | ctrl+s save | enter adds a new line in Models | esc cancel | ctrl+l toggle lang")
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
	return m.providerPaneLayout().body
}

func (m appModel) resultListView() string {
	return m.resultPaneLayout().body
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

func renderPaneTitle(title string, borderColor ...lipgloss.Color) string {
	resolvedBorderColor := resolvePaneBorderColor(borderColor...)
	return paneBorderStyle.Copy().Foreground(resolvedBorderColor).Bold(true).Render("|" + title + "|")
}

func renderTitledPane(title string, width int, body string, borderColor ...lipgloss.Color) string {
	resolvedBorderColor := resolvePaneBorderColor(borderColor...)
	rendered := paneStyle.Copy().BorderForeground(resolvedBorderColor).Width(width).Render(body)
	return renderTitledPaneFromRendered(title, rendered, resolvedBorderColor)
}

func renderTitledPaneFromRendered(title, rendered string, borderColor lipgloss.Color) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	border := lipgloss.RoundedBorder()
	titleText := renderPaneTitle(title, borderColor)
	visibleTitleWidth := lipgloss.Width("|" + title + "|")
	interiorWidth := maxInt(0, lipgloss.Width(lines[0])-2)
	leftRun := 1
	rightRun := interiorWidth - leftRun - visibleTitleWidth
	if rightRun < 0 {
		rightRun = 0
		leftRun = maxInt(0, interiorWidth-visibleTitleWidth)
	}

	lines[0] = strings.Join([]string{
		paneBorderStyle.Copy().Foreground(borderColor).Render(border.TopLeft + strings.Repeat(border.Top, leftRun)),
		titleText,
		paneBorderStyle.Copy().Foreground(borderColor).Render(strings.Repeat(border.Top, rightRun) + border.TopRight),
	}, "")

	return strings.Join(lines, "\n")
}

func renderTitledPaneWithHeight(title string, width, height int, body string, borderColor ...lipgloss.Color) string {
	resolvedBorderColor := resolvePaneBorderColor(borderColor...)

	if height <= 0 {
		return renderTitledPane(title, width, body, resolvedBorderColor)
	}

	lines, _ := layoutPaneBodyForHeight(width, height, body)
	rendered := paneStyle.Copy().BorderForeground(resolvedBorderColor).Width(width).Render(strings.Join(lines, "\n"))
	return renderTitledPaneFromRendered(title, rendered, resolvedBorderColor)
}

func renderScrollableTitledPaneWithHeight(title string, width, height int, body string, viewportOffset int, borderColor ...lipgloss.Color) string {
	resolvedBorderColor := resolvePaneBorderColor(borderColor...)

	if height <= 0 {
		return renderTitledPane(title, width, body, resolvedBorderColor)
	}

	lines, scrollbar := layoutPaneBodyForHeightAndOffset(width, height, body, viewportOffset)
	rendered := paneStyle.Copy().BorderForeground(resolvedBorderColor).Width(width).Render(strings.Join(lines, "\n"))
	titledPane := renderTitledPaneFromRendered(title, rendered, resolvedBorderColor)
	return rewriteRenderedPaneRightBorderWithScrollbar(titledPane, resolvedBorderColor, scrollbar)
}

func renderTitledPaneWithHeightAndRightScrollbar(title string, width, height int, body string, borderColor ...lipgloss.Color) string {
	return renderScrollableTitledPaneWithHeight(title, width, height, body, 0, borderColor...)
}

func renderTitledPaneWithHeightAndRightScrollbarViewport(title string, width, height int, body string, viewportOffset int, borderColor ...lipgloss.Color) string {
	return renderScrollableTitledPaneWithHeight(title, width, height, body, viewportOffset, borderColor...)
}

func layoutPaneBodyForHeight(width, height int, body string) ([]string, paneScrollbarMeta) {
	return layoutPaneBodyForHeightAndOffset(width, height, body, 0)
}

func layoutPaneBodyForHeightAndOffset(width, height int, body string, viewportOffset int) ([]string, paneScrollbarMeta) {
	contentHeight := maxInt(0, height-paneVerticalChrome)
	wrappedLines := wrapPaneContentLines(paneContentWidth(width), body)
	visibleLines := minInt(len(wrappedLines), contentHeight)
	maxOffset := maxInt(0, len(wrappedLines)-contentHeight)
	viewportOffset = maxInt(0, minInt(viewportOffset, maxOffset))
	layoutLines := append([]string(nil), wrappedLines...)

	switch {
	case len(layoutLines) > contentHeight:
		layoutLines = layoutLines[viewportOffset:minInt(viewportOffset+contentHeight, len(layoutLines))]
	case len(layoutLines) < contentHeight:
		layoutLines = append(layoutLines, make([]string, contentHeight-len(layoutLines))...)
	}

	return layoutLines, paneScrollbarMeta{
		totalLines:     len(wrappedLines),
		visibleLines:   visibleLines,
		viewportOffset: viewportOffset,
	}
}

func splitWrappedPaneLines(wrappedBody string) []string {
	if wrappedBody == "" {
		return nil
	}
	return strings.Split(wrappedBody, "\n")
}

func rewriteRenderedPaneRightBorderWithScrollbar(rendered string, borderColor lipgloss.Color, scrollbar paneScrollbarMeta) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) <= 2 {
		return rendered
	}

	thumbStart, thumbEnd, ok := scrollbar.thumbRange(len(lines) - 2)
	if !ok {
		return rendered
	}

	trackGlyph := lipgloss.RoundedBorder().Right
	for row := 1; row < len(lines)-1; row++ {
		glyph := trackGlyph
		if row-1 >= thumbStart && row-1 < thumbEnd {
			glyph = "▌"
		}
		lines[row] = replaceRenderedPaneRightBorderGlyph(lines[row], glyph, borderColor)
	}

	return strings.Join(lines, "\n")
}

func replaceRenderedPaneRightBorderGlyph(line, glyph string, borderColor lipgloss.Color) string {
	suffix := trailingANSIPattern.FindString(line)
	base := line[:len(line)-len(suffix)]
	if base == "" {
		return line
	}

	_, lastRuneSize := utf8.DecodeLastRuneInString(base)
	if lastRuneSize <= 0 {
		return line
	}

	return base[:len(base)-lastRuneSize] + paneBorderStyle.Copy().Foreground(borderColor).Render(glyph) + suffix
}

func (m paneScrollbarMeta) thumbRange(trackHeight int) (int, int, bool) {
	if trackHeight <= 0 || m.totalLines <= 0 || m.visibleLines <= 0 || m.totalLines <= m.visibleLines {
		return 0, 0, false
	}

	visibleLines := minInt(m.visibleLines, m.totalLines)
	thumbHeight := maxInt(1, int(math.Ceil(float64(trackHeight*visibleLines)/float64(m.totalLines))))
	thumbHeight = minInt(trackHeight, thumbHeight)

	maxOffset := maxInt(0, m.totalLines-visibleLines)
	viewportOffset := maxInt(0, minInt(m.viewportOffset, maxOffset))
	if maxOffset == 0 || thumbHeight >= trackHeight {
		return 0, thumbHeight, true
	}

	travel := trackHeight - thumbHeight
	thumbStart := int(math.Round(float64(travel*viewportOffset) / float64(maxOffset)))
	thumbStart = maxInt(0, minInt(thumbStart, travel))
	return thumbStart, thumbStart + thumbHeight, true
}

func resolvePaneBorderColor(borderColor ...lipgloss.Color) lipgloss.Color {
	if len(borderColor) > 0 {
		return borderColor[0]
	}
	return defaultPaneBorderColor
}

func wrapPaneBody(width int, body string) string {
	return wrapPaneContent(paneContentWidth(width), body)
}

func wrapPaneContentLines(contentWidth int, body string) []string {
	return splitWrappedPaneLines(wrapPaneContent(contentWidth, body))
}

func wrapPaneContent(contentWidth int, body string) string {
	if body == "" {
		return ""
	}

	if contentWidth <= 0 {
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

func (m appModel) requestModeInputView() string {
	choices := []string{
		m.renderRequestModeChoice(requestModeResponses, m.tr("chatgpt response", "chatgpt response")),
		m.renderRequestModeChoice(requestModeCompatible, m.tr("chatgpt compatible", "chatgpt compatible")),
	}
	prefix := "  "
	if m.focusIndex == requestSettingsModeField {
		prefix = selectionStyle.Render("> ")
	}
	return prefix + strings.Join(choices, " / ")
}

func (m appModel) renderRequestModeChoice(mode requestMode, label string) string {
	if m.requestMode == mode {
		return selectionStyle.Render("[" + label + "]")
	}
	return mutedStyle.Render(label)
}

func (m appModel) formPaneBorderColor() lipgloss.Color {
	if m.mode == requestSettingsMode {
		return requestPaneBorderColor
	}
	return addProviderPaneBorderColor
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
