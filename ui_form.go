package main

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func newInputs(lang appLanguage) []textinput.Model {
	inputs := make([]textinput.Model, len(formFields))
	for i, field := range formFields {
		inputs[i] = newInput(field.kind)
	}
	applyInputLocale(inputs, lang, formPaneWidth(0))
	inputs[0].Focus()
	return inputs
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
	case inputKindModels:
		input.CharLimit = 0
	}
	return input
}

func (m appModel) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		if i == m.focusIndex {
			m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *appModel) resetForm() {
	for i := range m.inputs {
		m.inputs[i].SetValue("")
		m.inputs[i].Blur()
	}
	m.focusIndex = 0
	m.applyPlaceholders()
	m.inputs[0].Focus()
}

func (m *appModel) cycleFocus(direction string) {
	m.inputs[m.focusIndex].Blur()
	if direction == "shift+tab" || direction == "up" {
		m.focusIndex--
	} else {
		m.focusIndex++
	}
	if m.focusIndex < 0 {
		m.focusIndex = len(m.inputs) - 1
	}
	if m.focusIndex >= len(m.inputs) {
		m.focusIndex = 0
	}
	m.inputs[m.focusIndex].Focus()
}
