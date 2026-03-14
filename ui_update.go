package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m appModel) Init() tea.Cmd {
	return tea.DisableMouse
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyPlaceholders()
		return m, nil
	case spinner.TickMsg:
		if !m.running {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case runFinishedMsg:
		m.finishRun(msg.Results)
		return m, nil
	case tea.KeyMsg:
		if key := msg.String(); key == "ctrl+c" {
			return m, tea.Quit
		} else if key == "l" {
			m.toggleLanguage()
			return m, nil
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

func (m appModel) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "up", "k", "down", "j", "a", "enter":
		if m.running {
			m.setStatus(statusWarning, m.tr("检测进行中，请等待完成后再切换或操作。", "Checks are still running. Wait for completion before changing providers."))
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
		m.enterAddMode()
	case "enter":
		if len(m.config.Providers) == 0 {
			m.setStatus(statusWarning, m.tr("请先新增至少一个供应商后再检测。", "Add at least one provider before running checks."))
			return m, nil
		}
		return m, tea.Batch(m.spinner.Tick, m.startChecks())
	}

	return m, nil
}

func (m appModel) handleFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cancelAddMode()
		return m, nil
	case "tab", "shift+tab", "up", "down":
		m.cycleFocus(msg.String())
		return m, nil
	case "enter":
		provider, err := m.buildProviderFromInputs()
		if err != nil {
			m.setStatus(statusError, err.Error())
			return m, nil
		}
		if err := m.saveProvider(provider); err != nil {
			m.setStatus(statusError, fmt.Sprintf(m.tr("保存配置失败：%v", "Save config failed: %v"), err))
			return m, nil
		}

		m.mode = listMode
		m.cursor = len(m.config.Providers) - 1
		m.resetForm()
		m.setStatus(statusSuccess, fmt.Sprintf(m.tr("已保存供应商 %s，共 %d 个模型。", "Saved provider %s with %d model(s)."), provider.BaseURL, len(provider.Models)))
		return m, nil
	}

	return m.updateInputs(msg)
}

func (m *appModel) finishRun(results []modelResult) {
	m.running = false
	m.results = results
	failures := 0
	for _, result := range results {
		if result.Error != "" {
			failures++
		}
	}
	if len(results) == 0 {
		m.setStatus(statusWarning, m.tr("当前供应商没有可检测模型。", "Selected provider has no models."))
	} else if failures == 0 {
		m.setStatus(statusSuccess, fmt.Sprintf(m.tr("已完成 %d 个模型检测。", "Finished %d model checks."), len(results)))
	} else {
		m.setStatus(statusWarning, fmt.Sprintf(m.tr("检测完成，错误 %d/%d。", "Finished with %d/%d errors."), failures, len(results)))
	}
}

func (m *appModel) enterAddMode() {
	m.mode = addMode
	m.resetForm()
	m.setStatus(statusInfo, m.tr("新增供应商：回车保存，Esc 取消。", "Add a provider. Press Enter to save or Esc to cancel."))
}

func (m *appModel) cancelAddMode() {
	m.mode = listMode
	m.setStatus(statusInfo, m.tr("已取消新增供应商。", "Canceled adding provider."))
}

func (m *appModel) startChecks() tea.Cmd {
	selected := m.config.Providers[m.cursor]
	m.running = true
	m.results = nil
	if m.lang == langEN {
		m.setStatus(statusLoading, fmt.Sprintf("Checking %d model(s) from %s with concurrency %d...", len(selected.Models), selected.BaseURL, m.concurrency))
	} else {
		m.setStatus(statusLoading, fmt.Sprintf("正在检测 %s 的 %d 个模型（并发 %d）...", selected.BaseURL, len(selected.Models), m.concurrency))
	}
	return runChecksCmd(selected, m.concurrency)
}

func (m *appModel) buildProviderFromInputs() (provider, error) {
	baseURL, err := normalizeBaseURL(m.inputs[0].Value())
	if err != nil {
		return provider{}, fmt.Errorf(m.tr("URL 无效：%v", "Invalid URL: %v"), err)
	}
	models := splitModels(m.inputs[2].Value())
	if len(models) == 0 {
		return provider{}, errors.New(m.tr("至少填写一个模型。", "At least one model is required."))
	}

	return provider{
		BaseURL: baseURL,
		APIKey:  strings.TrimSpace(m.inputs[1].Value()),
		Models:  models,
	}, nil
}

func (m *appModel) saveProvider(provider provider) error {
	m.config.Providers = append(m.config.Providers, provider)
	if err := saveConfig(m.configPath, m.config); err != nil {
		m.config.Providers = m.config.Providers[:len(m.config.Providers)-1]
		return err
	}
	return nil
}
