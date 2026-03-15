package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type formScrollDirection int

const (
	formScrollDirectionNeutral formScrollDirection = iota
	formScrollDirectionUp
	formScrollDirectionDown
)

const formScrollLeadRows = 2

func newProviderInputs(lang appLanguage) (textinput.Model, textinput.Model, textarea.Model) {
	baseURLInput := newInput(inputKindText)
	apiKeyInput := newInput(inputKindPassword)
	modelsInput := newModelsInput()
	applyFormLocale(&baseURLInput, &apiKeyInput, &modelsInput, lang, formPaneWidth(0))
	baseURLInput.Focus()
	return baseURLInput, apiKeyInput, modelsInput
}

func newPromptInput() textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 0
	input.TextStyle = inputStyle
	input.SetValue(juicyPrompt)
	input.Blur()
	return input
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

func (m appModel) updateModelsInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	paneWidth := m.activeFormPaneWidth()
	oldHeight := m.modelsInput.Height()
	scrollDirection := formScrollDirectionNeutral
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		scrollDirection = formScrollDirectionForKey(keyMsg.String())
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
	if fieldIndex < addProviderBaseURLField || fieldIndex >= addProviderFieldCount {
		return 0
	}

	labelRows := len(wrapPaneContentLines(paneContentWidth(paneWidth), renderFieldLabel(formFields[fieldIndex].label.forLang(m.lang))))
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
	if m.focusIndex < addProviderBaseURLField || m.focusIndex >= addProviderFieldCount {
		return 0
	}
	return m.formFieldStartRow(m.focusIndex, paneWidth) + m.formFieldActiveCursorRow(m.focusIndex, paneWidth)
}

func (m *appModel) syncFormPaneScroll() {
	m.syncFormPaneScrollWithDirection(formScrollDirectionNeutral)
}

func (m *appModel) syncFormPaneScrollWithDirection(direction formScrollDirection) {
	if m.mode != addMode {
		m.formPaneScrollOffset = 0
		return
	}

	layout := m.formPaneLayout()
	visibleHeight := m.formPaneVisibleContentHeight()
	if visibleHeight <= 0 {
		m.formPaneScrollOffset = 0
		return
	}

	maxOffset := maxInt(0, len(layout.wrappedLines)-visibleHeight)
	offset := maxInt(0, minInt(m.formPaneScrollOffset, maxOffset))
	if layout.activeCursorRow < 0 {
		m.formPaneScrollOffset = offset
		return
	}

	activeCursorRow := maxInt(0, minInt(layout.activeCursorRow, len(layout.wrappedLines)-1))
	anchorRow := maxInt(0, minInt(m.formScrollAnchorRow(listPaneWidth(m.width), direction), len(layout.wrappedLines)-1))
	margin := formScrollMargin(visibleHeight)

	switch direction {
	case formScrollDirectionUp:
		topThreshold := offset + margin
		if anchorRow < topThreshold {
			offset = anchorRow - margin
		}
	case formScrollDirectionDown:
		bottomThreshold := offset + visibleHeight - 1 - margin
		if activeCursorRow > bottomThreshold {
			offset = activeCursorRow - (visibleHeight - 1 - margin)
		}
	}

	if direction == formScrollDirectionUp && anchorRow < offset {
		offset = anchorRow
	}
	if activeCursorRow < offset {
		offset = activeCursorRow
	}
	if activeCursorRow >= offset+visibleHeight {
		offset = activeCursorRow - visibleHeight + 1
	}
	m.formPaneScrollOffset = maxInt(0, minInt(offset, maxOffset))
}

func formScrollMargin(visibleHeight int) int {
	if visibleHeight <= formScrollLeadRows*2+1 {
		return 0
	}
	return formScrollLeadRows
}

func formScrollDirectionForKey(key string) formScrollDirection {
	switch key {
	case "up", "shift+tab":
		return formScrollDirectionUp
	case "down", "tab":
		return formScrollDirectionDown
	default:
		return formScrollDirectionNeutral
	}
}

func (m appModel) formScrollAnchorRow(paneWidth int, direction formScrollDirection) int {
	if direction != formScrollDirectionUp {
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
	if index < addProviderBaseURLField || index >= addProviderFieldCount {
		return
	}
	if m.focusIndex != index {
		m.blurFocusedInput()
		m.focusIndex = index
	}
	m.focusCurrentInput()
}

func (m appModel) handleVerticalFormNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	direction := formScrollDirectionForKey(msg.String())

	switch m.focusIndex {
	case addProviderBaseURLField:
		if direction == formScrollDirectionUp {
			m.setFormFocus(addProviderModelsField)
			m.moveModelsCursorToLastRow()
			m.scrollFormPaneToBottom()
			return m, nil
		}
	case addProviderAPIKeyField:
		if direction == formScrollDirectionDown {
			m.setFormFocus(addProviderModelsField)
			m.moveModelsCursorToFirstRow()
			m.syncFormPaneScrollWithDirection(direction)
			return m, nil
		}
	case addProviderModelsField:
		if direction == formScrollDirectionUp && modelsInputIsFirstRow(m.modelsInput) {
			m.setFormFocus(addProviderAPIKeyField)
			m.syncFormPaneScrollWithDirection(direction)
			return m, nil
		}
		if direction == formScrollDirectionDown && modelsInputIsLastRow(m.modelsInput) {
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
	m.syncFormPaneScrollWithDirection(formScrollDirectionUp)
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
	if m.focusIndex < 0 {
		m.focusIndex = addProviderFieldCount - 1
	}
	if m.focusIndex >= addProviderFieldCount {
		m.focusIndex = 0
	}
	m.focusCurrentInput()
	m.syncFormPaneScrollWithDirection(formScrollDirectionForKey(direction))
}

func (m *appModel) blurFocusedInput() {
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
	switch m.focusIndex {
	case addProviderBaseURLField:
		m.baseURLInput.Focus()
	case addProviderAPIKeyField:
		m.apiKeyInput.Focus()
	case addProviderModelsField:
		m.modelsInput.Focus()
	}
}
