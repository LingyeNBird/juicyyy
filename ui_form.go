package main

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newProviderInputs(lang appLanguage) (textinput.Model, textinput.Model, textarea.Model) {
	baseURLInput := newInput(inputKindText)
	apiKeyInput := newInput(inputKindPassword)
	modelsInput := newModelsInput()
	applyFormLocale(&baseURLInput, &apiKeyInput, &modelsInput, lang, formPaneWidth(0))
	baseURLInput.Focus()
	return baseURLInput, apiKeyInput, modelsInput
}

func newRequestSettingsInputs(lang appLanguage, settings requestSettings) (textinput.Model, textinput.Model, textinput.Model) {
	settings = normalizeRequestSettings(settings)
	promptInput := newInput(inputKindText)
	promptInput.CharLimit = 0
	promptInput.SetValue(settings.Prompt)
	promptInput.Focus()

	timeoutInput := newInput(inputKindText)
	timeoutInput.SetValue(strconv.Itoa(settings.TimeoutSeconds))
	timeoutInput.Blur()

	retryInput := newInput(inputKindText)
	retryInput.SetValue(strconv.Itoa(settings.RetryCount))
	retryInput.Blur()

	applyRequestSettingsLocale(&promptInput, &timeoutInput, &retryInput, lang, formPaneWidth(0))
	return promptInput, timeoutInput, retryInput
}

func newInput(kind inputKind) textinput.Model {
	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = defaultInputCharLimit
	input.Width = inputWidthForFormPane(formPaneWidth(0))
	input.TextStyle = inputStyle
	switch kind {
	case inputKindPassword:
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '*'
	}
	return input
}

func newModelsInput() textarea.Model {
	return newStyledModelsInput("", "", false, formPaneWidth(0))
}

func newStyledModelsInput(value, placeholder string, focused bool, paneWidth int) textarea.Model {
	input := textarea.New()
	input.CharLimit = 0
	input.ShowLineNumbers = false
	input.FocusedStyle.Text = inputStyle
	input.FocusedStyle.CursorLine = inputStyle
	input.BlurredStyle.Text = inputStyle
	input.BlurredStyle.CursorLine = inputStyle
	input.Placeholder = placeholder
	if value != "" {
		input.SetValue(value)
	}
	if focused {
		input.Focus()
	} else {
		input.Blur()
	}
	syncModelsInputLayout(&input, paneWidth)
	return input
}

func syncModelsInputLayout(input *textarea.Model, paneWidth int) {
	input.Prompt = modelsInputPrompt
	input.SetPromptFunc(lipgloss.Width(modelsInputPrompt), func(lineIdx int) string {
		if lineIdx == 0 {
			return modelsInputPrompt
		}
		return modelsInputIndent
	})
	input.SetWidth(modelsInputWidthForPane(paneWidth))
	input.SetHeight(modelsInputHeightForValue(input.Value(), paneWidth))
}

func (m appModel) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.mode == requestSettingsMode {
		return m.updateRequestSettingsInputs(msg)
	}

	switch m.focusIndex {
	case addProviderBaseURLField:
		var cmd tea.Cmd
		m.baseURLInput, cmd = m.baseURLInput.Update(msg)
		m.syncFormPaneScroll()
		return m, cmd
	case addProviderAPIKeyField:
		var cmd tea.Cmd
		m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
		m.syncFormPaneScroll()
		return m, cmd
	case addProviderModelsField:
		return m.updateModelsInput(msg)
	}
	return m, nil
}

func (m appModel) updateRequestSettingsInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.focusIndex {
	case requestSettingsPromptField:
		var cmd tea.Cmd
		m.requestPromptInput, cmd = m.requestPromptInput.Update(msg)
		m.syncFormPaneScroll()
		return m, cmd
	case requestSettingsTimeoutField:
		var cmd tea.Cmd
		m.requestTimeoutInput, cmd = m.requestTimeoutInput.Update(msg)
		m.syncFormPaneScroll()
		return m, cmd
	case requestSettingsRetryField:
		var cmd tea.Cmd
		m.requestRetryInput, cmd = m.requestRetryInput.Update(msg)
		m.syncFormPaneScroll()
		return m, cmd
	case requestSettingsModeField:
		m.syncFormPaneScroll()
		return m, nil
	}
	return m, nil
}

func (m appModel) updateModelsInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	paneWidth := m.activeFormPaneWidth()
	oldHeight := m.modelsInput.Height()
	scrollDirection := paneScrollDirectionNeutral
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		scrollDirection = paneScrollDirectionForKey(keyMsg.String())
	}

	var cmd tea.Cmd
	m.modelsInput, cmd = m.modelsInput.Update(msg)
	syncModelsInputLayout(&m.modelsInput, paneWidth)

	if modelsInputNeedsViewportReset(m.modelsInput, oldHeight) {
		m.modelsInput = rebuildModelsInput(m.modelsInput, paneWidth)
	}
	m.syncFormPaneScrollWithDirection(scrollDirection)

	return m, cmd
}

func modelsInputNeedsViewportReset(input textarea.Model, oldHeight int) bool {
	if input.Height() <= oldHeight {
		return false
	}
	return modelsInputWrappedCursorRow(input) >= oldHeight
}

func modelsInputWrappedCursorRow(input textarea.Model) int {
	row := 0
	for _, line := range strings.Split(input.Value(), "\n")[:input.Line()] {
		row += len(wrapTextareaLine([]rune(line), input.Width()))
	}
	return row + input.LineInfo().RowOffset
}

func modelsInputCursorColumn(input textarea.Model) int {
	lineInfo := input.LineInfo()
	return lineInfo.StartColumn + lineInfo.ColumnOffset
}

func modelsInputIsFirstRow(input textarea.Model) bool {
	return input.Line() == 0 && input.LineInfo().RowOffset == 0
}

func modelsInputIsLastRow(input textarea.Model) bool {
	return modelsInputWrappedCursorRow(input) >= maxInt(0, wrappedVisibleRowCount(input.Value(), input.Width())-1)
}

func moveModelsInputCursorToFirstRow(input *textarea.Model, paneWidth int) {
	syncModelsInputLayout(input, paneWidth)
	restoreModelsInputCursor(input, 0, 0)
}

func moveModelsInputCursorToLastRow(input *textarea.Model, paneWidth int) {
	syncModelsInputLayout(input, paneWidth)
	lines := strings.Split(input.Value(), "\n")
	lastLine := maxInt(0, len(lines)-1)
	lastCol := len([]rune(lines[lastLine]))
	restoreModelsInputCursor(input, lastLine, lastCol)
}

func (m appModel) formFieldInputCursorRow(fieldIndex, paneWidth int) int {
	if m.mode == requestSettingsMode {
		return 0
	}

	switch fieldIndex {
	case addProviderBaseURLField, addProviderAPIKeyField:
		return 0
	case addProviderModelsField:
		input := m.modelsInput
		syncModelsInputLayout(&input, paneWidth)
		return modelsInputWrappedCursorRow(input)
	default:
		return 0
	}
}

func (m appModel) formFieldActiveCursorRow(fieldIndex, paneWidth int) int {
	fields := formFields
	if m.mode == requestSettingsMode {
		if fieldIndex < requestSettingsPromptField || fieldIndex >= requestSettingsFieldCount {
			return 0
		}
		fields = requestSettingsFields
	} else if fieldIndex < addProviderBaseURLField || fieldIndex >= addProviderFieldCount {
		return 0
	}

	labelRows := len(wrapPaneContentLines(paneContentWidth(paneWidth), renderFieldLabel(fields[fieldIndex].label.forLang(m.lang))))
	return labelRows + m.formFieldInputCursorRow(fieldIndex, paneWidth)
}

func (m appModel) formFieldStartRow(fieldIndex, paneWidth int) int {
	sectionIndex := fieldIndex + 1
	sections := m.formPaneSections(paneWidth)
	contentWidth := paneContentWidth(paneWidth)
	startRow := 0

	for i, section := range sections {
		if i > 0 {
			startRow++
		}
		if i == sectionIndex {
			return startRow
		}
		startRow += len(wrapPaneContentLines(contentWidth, section))
	}

	return 0
}

func (m appModel) activeFormCursorRow(paneWidth int) int {
	fieldCount := addProviderFieldCount
	if m.mode == requestSettingsMode {
		fieldCount = requestSettingsFieldCount
	}
	if m.focusIndex < 0 || m.focusIndex >= fieldCount {
		return 0
	}
	return m.formFieldStartRow(m.focusIndex, paneWidth) + m.formFieldActiveCursorRow(m.focusIndex, paneWidth)
}

func (m *appModel) syncFormPaneScroll() {
	m.syncFormPaneScrollWithDirection(paneScrollDirectionNeutral)
}

func (m *appModel) syncFormPaneScrollWithDirection(direction paneScrollDirection) {
	if m.mode == listMode {
		m.formPaneScrollOffset = 0
		return
	}

	layout := m.formPaneLayout()
	visibleHeight := m.formPaneVisibleContentHeight()
	anchorRow := m.formScrollAnchorRow(listPaneWidth(m.width), direction)
	m.formPaneScrollOffset = syncPaneScrollOffset(layout, visibleHeight, m.formPaneScrollOffset, direction, anchorRow)
}

func (m appModel) formScrollAnchorRow(paneWidth int, direction paneScrollDirection) int {
	if m.mode == requestSettingsMode {
		return m.activeFormCursorRow(paneWidth)
	}

	if direction != paneScrollDirectionUp {
		return m.activeFormCursorRow(paneWidth)
	}

	switch m.focusIndex {
	case addProviderBaseURLField:
		return 0
	case addProviderAPIKeyField:
		return m.formFieldStartRow(addProviderAPIKeyField, paneWidth)
	default:
		return m.activeFormCursorRow(paneWidth)
	}
}

func (m *appModel) moveModelsCursorToFirstRow() {
	moveModelsInputCursorToFirstRow(&m.modelsInput, m.activeFormPaneWidth())
}

func (m *appModel) moveModelsCursorToLastRow() {
	moveModelsInputCursorToLastRow(&m.modelsInput, m.activeFormPaneWidth())
}

func (m *appModel) setFormFocus(index int) {
	fieldCount := addProviderFieldCount
	if m.mode == requestSettingsMode {
		fieldCount = requestSettingsFieldCount
	}
	if index < 0 || index >= fieldCount {
		return
	}
	if m.focusIndex != index {
		m.blurFocusedInput()
		m.focusIndex = index
	}
	m.focusCurrentInput()
}

func (m appModel) handleVerticalFormNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == requestSettingsMode {
		m.cycleFocus(msg.String())
		return m, nil
	}

	direction := paneScrollDirectionForKey(msg.String())

	switch m.focusIndex {
	case addProviderBaseURLField:
		if direction == paneScrollDirectionUp {
			m.setFormFocus(addProviderModelsField)
			m.moveModelsCursorToLastRow()
			m.scrollFormPaneToBottom()
			return m, nil
		}
	case addProviderAPIKeyField:
		if direction == paneScrollDirectionDown {
			m.setFormFocus(addProviderModelsField)
			m.moveModelsCursorToFirstRow()
			m.syncFormPaneScrollWithDirection(direction)
			return m, nil
		}
	case addProviderModelsField:
		if direction == paneScrollDirectionUp && modelsInputIsFirstRow(m.modelsInput) {
			m.setFormFocus(addProviderAPIKeyField)
			m.syncFormPaneScrollWithDirection(direction)
			return m, nil
		}
		if direction == paneScrollDirectionDown && modelsInputIsLastRow(m.modelsInput) {
			m.setFormFocus(addProviderBaseURLField)
			m.scrollFormPaneToTop()
			return m, nil
		}
		return m.updateModelsInput(msg)
	}

	m.cycleFocus(msg.String())
	return m, nil
}

func (m *appModel) scrollFormPaneToTop() {
	m.formPaneScrollOffset = 0
	m.syncFormPaneScrollWithDirection(paneScrollDirectionUp)
}

func (m *appModel) scrollFormPaneToBottom() {
	layout := m.formPaneLayout()
	visibleHeight := m.formPaneVisibleContentHeight()
	m.formPaneScrollOffset = maxInt(0, len(layout.wrappedLines)-visibleHeight)
}

func rebuildModelsInput(input textarea.Model, paneWidth int) textarea.Model {
	rebuilt := newStyledModelsInput(input.Value(), input.Placeholder, input.Focused(), paneWidth)
	restoreModelsInputCursor(&rebuilt, input.Line(), modelsInputCursorColumn(input))
	return rebuilt
}

func restoreModelsInputCursor(input *textarea.Model, targetRow, targetCol int) {
	if targetRow < 0 {
		targetRow = 0
	}
	if targetCol < 0 {
		targetCol = 0
	}
	if lineCount := input.LineCount(); lineCount > 0 && targetRow >= lineCount {
		targetRow = lineCount - 1
	}

	for input.Line() > 0 {
		input.CursorUp()
	}
	input.CursorStart()

	for step, maxSteps := 0, wrappedVisibleRowCount(input.Value(), input.Width()); input.Line() < targetRow && step < maxSteps; step++ {
		input.CursorDown()
	}
	input.SetCursor(targetCol)
}

func (m *appModel) resetForm() {
	m.baseURLInput.SetValue("")
	m.baseURLInput.Blur()
	m.apiKeyInput.SetValue("")
	m.apiKeyInput.Blur()
	m.modelsInput.SetValue("")
	m.modelsInput.Blur()
	m.editingIndex = noEditingProviderIndex
	m.focusIndex = 0
	m.formPaneScrollOffset = 0
	m.applyPlaceholders()
	m.baseURLInput.Focus()
}

func (m *appModel) preloadRequestSettingsForm(settings requestSettings) {
	settings = normalizeRequestSettings(settings)
	m.requestPromptInput.SetValue(settings.Prompt)
	m.requestPromptInput.Blur()
	m.requestTimeoutInput.SetValue(strconv.Itoa(settings.TimeoutSeconds))
	m.requestTimeoutInput.Blur()
	m.requestRetryInput.SetValue(strconv.Itoa(settings.RetryCount))
	m.requestRetryInput.Blur()
	m.requestMode = settings.Mode
	m.focusIndex = requestSettingsPromptField
	m.formPaneScrollOffset = 0
	m.applyPlaceholders()
	m.requestPromptInput.Focus()
}

func (m *appModel) preloadForm(provider provider) {
	m.baseURLInput.SetValue(provider.BaseURL)
	m.baseURLInput.Blur()
	m.apiKeyInput.SetValue(provider.APIKey)
	m.apiKeyInput.Blur()
	m.modelsInput.SetValue(strings.Join(provider.Models, "\n"))
	m.modelsInput.Blur()
	m.focusIndex = addProviderBaseURLField
	m.formPaneScrollOffset = 0
	m.applyPlaceholders()
	m.baseURLInput.Focus()
}

func (m *appModel) cycleFocus(direction string) {
	m.blurFocusedInput()
	if direction == "shift+tab" || direction == "up" {
		m.focusIndex--
	} else {
		m.focusIndex++
	}
	fieldCount := addProviderFieldCount
	if m.mode == requestSettingsMode {
		fieldCount = requestSettingsFieldCount
	}
	if m.focusIndex < 0 {
		m.focusIndex = fieldCount - 1
	}
	if m.focusIndex >= fieldCount {
		m.focusIndex = 0
	}
	m.focusCurrentInput()
	m.syncFormPaneScrollWithDirection(paneScrollDirectionForKey(direction))
}

func (m *appModel) blurFocusedInput() {
	if m.mode == requestSettingsMode {
		switch m.focusIndex {
		case requestSettingsPromptField:
			m.requestPromptInput.Blur()
		case requestSettingsTimeoutField:
			m.requestTimeoutInput.Blur()
		case requestSettingsRetryField:
			m.requestRetryInput.Blur()
		}
		return
	}

	switch m.focusIndex {
	case addProviderBaseURLField:
		m.baseURLInput.Blur()
	case addProviderAPIKeyField:
		m.apiKeyInput.Blur()
	case addProviderModelsField:
		m.modelsInput.Blur()
	}
}

func (m *appModel) focusCurrentInput() {
	if m.mode == requestSettingsMode {
		switch m.focusIndex {
		case requestSettingsPromptField:
			m.requestPromptInput.Focus()
		case requestSettingsTimeoutField:
			m.requestTimeoutInput.Focus()
		case requestSettingsRetryField:
			m.requestRetryInput.Focus()
		}
		return
	}

	switch m.focusIndex {
	case addProviderBaseURLField:
		m.baseURLInput.Focus()
	case addProviderAPIKeyField:
		m.apiKeyInput.Focus()
	case addProviderModelsField:
		m.modelsInput.Focus()
	}
}
