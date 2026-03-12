package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

type model struct {
	config       appConfig
	configPath   string
	mode         viewMode
	cursor       int
	activeResult int
	inputs       []textinput.Model
	focusIndex   int
	width        int
	height       int
	status       string
	results      []modelResult
	running      bool
	spinner      spinner.Model
	concurrency  int
}

func newModel(cfg appConfig, configPath string) model {
	spin := spinner.New()
	spin.Spinner = spinner.Line
	spin.Style = goodStyle

	inputs := []textinput.Model{
		newInput("OAI base URL or /chat/completions URL"),
		newPasswordInput("API key"),
		newInput("Models (comma separated)"),
	}
	inputs[0].Focus()

	return model{
		config:      cfg,
		configPath:  configPath,
		mode:        listMode,
		inputs:      inputs,
		status:      fmt.Sprintf("Config file: %s", configPath),
		spinner:     spin,
		concurrency: 5,
	}
}

func newInput(placeholder string) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.Prompt = "> "
	input.CharLimit = 512
	input.Width = 56
	input.TextStyle = inputStyle
	return input
}

func newPasswordInput(placeholder string) textinput.Model {
	input := newInput(placeholder)
	input.EchoMode = textinput.EchoPassword
	input.EchoCharacter = '*'
	return input
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case spinner.TickMsg:
		if !m.running {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case runFinishedMsg:
		m.running = false
		m.results = msg.Results
		failures := 0
		for _, result := range msg.Results {
			if result.Error != "" {
				failures++
			}
		}
		if len(msg.Results) == 0 {
			m.status = "Selected provider has no models."
		} else if failures == 0 {
			m.status = fmt.Sprintf("Finished %d model checks.", len(msg.Results))
		} else {
			m.status = fmt.Sprintf("Finished with %d/%d errors.", failures, len(msg.Results))
		}
		return m, nil
	case tea.KeyMsg:
		if key := msg.String(); key == "ctrl+c" {
			return m, tea.Quit
		}

		if m.mode == addMode {
			return m.handleFormKeys(msg)
		}
		return m.handleListKeys(msg)
	}

	if m.mode == addMode {
		return m.updateInputs(msg)
	}

	return m, nil
}

func (m model) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "up", "k", "down", "j", "a", "enter":
		if m.running {
			m.status = "Checks are still running. Wait for completion before changing providers."
			return m, nil
		}
	}

	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.config.Providers)-1 {
			m.cursor++
		}
	case "a":
		m.mode = addMode
		m.status = "Add a provider. Press Enter to save or Esc to cancel."
		m.resetForm()
	case "enter":
		if len(m.config.Providers) == 0 {
			m.status = "Add at least one provider before running checks."
			return m, nil
		}
		selected := m.config.Providers[m.cursor]
		m.running = true
		m.results = nil
		m.status = fmt.Sprintf("Checking %d model(s) from %s with concurrency %d...", len(selected.Models), selected.BaseURL, m.concurrency)
		return m, tea.Batch(m.spinner.Tick, runChecksCmd(selected, m.concurrency))
	}

	return m, nil
}

func (m model) handleFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = listMode
		m.status = "Canceled adding provider."
		return m, nil
	case "tab", "shift+tab", "up", "down":
		m.cycleFocus(msg.String())
		return m, nil
	case "enter":
		baseURL, err := normalizeBaseURL(m.inputs[0].Value())
		if err != nil {
			m.status = fmt.Sprintf("Invalid URL: %v", err)
			return m, nil
		}
		models := splitModels(m.inputs[2].Value())
		if len(models) == 0 {
			m.status = "At least one model is required."
			return m, nil
		}

		m.config.Providers = append(m.config.Providers, provider{
			BaseURL: baseURL,
			APIKey:  strings.TrimSpace(m.inputs[1].Value()),
			Models:  models,
		})
		if err := saveConfig(m.configPath, m.config); err != nil {
			m.status = fmt.Sprintf("Save config failed: %v", err)
			m.config.Providers = m.config.Providers[:len(m.config.Providers)-1]
			return m, nil
		}

		m.mode = listMode
		m.cursor = len(m.config.Providers) - 1
		m.status = fmt.Sprintf("Saved provider %s with %d model(s).", baseURL, len(models))
		m.resetForm()
		return m, nil
	}

	return m.updateInputs(msg)
}

func (m model) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		if i == m.focusIndex {
			m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *model) resetForm() {
	for i := range m.inputs {
		m.inputs[i].SetValue("")
		m.inputs[i].Blur()
	}
	m.focusIndex = 0
	m.inputs[0].Focus()
}

func (m *model) cycleFocus(direction string) {
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

func runChecksCmd(selected provider, concurrency int) tea.Cmd {
	return func() tea.Msg {
		results := runJuicyChecks(context.Background(), selected, concurrency)
		return runFinishedMsg{Results: results}
	}
}

func (m model) View() string {
	if m.mode == addMode {
		return m.formView()
	}
	return m.listView()
}

func (m model) listView() string {
	header := titleStyle.Render("Juicy Batch Checker") + "  " + mutedStyle.Render("Prompt: "+juicyPrompt)
	providerPane := paneStyle.Width(maxInt(36, m.width/2-3)).Render(m.providerListView())
	resultPane := paneStyle.Width(maxInt(36, m.width/2-3)).Render(m.resultListView())
	body := lipgloss.JoinHorizontal(lipgloss.Top, providerPane, resultPane)
	footer := helpStyle.Render("Keys: a add provider | enter run checks | j/k move | q quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		body,
		"",
		m.statusLine(),
		footer,
	)
}

func (m model) formView() string {
	lines := []string{
		titleStyle.Render("Add Provider"),
		"",
		"Fill in an OAI-compatible base URL, API key, and comma-separated models.",
		"",
		"Base URL",
		m.inputs[0].View(),
		"",
		"API Key",
		m.inputs[1].View(),
		"",
		"Models",
		m.inputs[2].View(),
		"",
		m.statusLine(),
		helpStyle.Render("Keys: tab/shift+tab move | enter save | esc cancel"),
	}

	return paneStyle.Width(maxInt(72, m.width-6)).Render(strings.Join(lines, "\n"))
}

func (m model) providerListView() string {
	if len(m.config.Providers) == 0 {
		return "Providers\n\nNo providers saved yet. Press 'a' to add one."
	}

	lines := []string{"Providers", ""}
	for i, provider := range m.config.Providers {
		cursor := "  "
		line := fmt.Sprintf("%s (%d models)", provider.BaseURL, len(provider.Models))
		if i == m.cursor {
			cursor = "> "
			line = goodStyle.Render(line)
		}
		lines = append(lines, cursor+line)
		lines = append(lines, mutedStyle.Render("   "+strings.Join(provider.Models, ", ")))
	}

	return strings.Join(lines, "\n")
}

func (m model) resultListView() string {
	lines := []string{"Results", ""}
	if m.running {
		lines = append(lines, m.spinner.View()+" Running juicy checks...")
	}
	if len(m.results) == 0 {
		if !m.running {
			lines = append(lines, "No results yet. Select a provider and press Enter.")
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

func (m model) statusLine() string {
	if strings.TrimSpace(m.status) == "" {
		return mutedStyle.Render("Ready")
	}
	return mutedStyle.Render(m.status)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
