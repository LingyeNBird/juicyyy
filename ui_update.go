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
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}

		if m.mode != addMode && m.promptEditing {
			return m.handlePromptKeys(msg)
		}

		if key == "ctrl+l" || (key == "l" && m.mode != addMode) {
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
	case "tab":
		m.focusPromptInput()
		return m, nil
	case "up", "k", "down", "j", "a", "e", "enter":
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
	case "e":
		if len(m.config.Providers) == 0 {
			m.setStatus(statusWarning, m.tr("还没有可编辑的供应商，请先新增。", "No providers to edit yet. Add one first."))
			return m, nil
		}
		m.enterEditMode(m.cursor)
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
		m.cancelFormMode()
		return m, nil
	case "tab", "shift+tab", "up", "down":
		if m.focusIndex == addProviderModelsField && (msg.String() == "up" || msg.String() == "down") {
			return m.updateInputs(msg)
		}
		m.cycleFocus(msg.String())
		return m, nil
	case "enter":
		if m.focusIndex == addProviderModelsField {
			return m.updateInputs(msg)
		}
		provider, err := m.buildProviderFromInputs()
		if err != nil {
			m.setStatus(statusError, err.Error())
			return m, nil
		}
		savedIndex, err := m.saveProvider(provider)
		if err != nil {
			m.setStatus(statusError, m.formSaveFailureStatus(err))
			return m, nil
		}
		successStatus := m.formSaveSuccessStatus(provider)

		m.mode = listMode
		m.cursor = savedIndex
		m.resetForm()
		m.setStatus(statusSuccess, successStatus)
		return m, nil
	}

	return m.updateInputs(msg)
}

func (m appModel) handlePromptKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "esc", "enter":
		m.blurPromptInput()
		return m, nil
	}

	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
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
	m.blurPromptInput()
	m.mode = addMode
	m.resetForm()
	m.setStatus(statusInfo, m.tr("新增供应商：模型支持逗号或换行；在基础 URL 或 API 密钥上按回车保存，Esc 取消。", "Add a provider. Models accept commas or new lines; press Enter on Base URL or API Key to save, or Esc to cancel."))
}

func (m *appModel) enterEditMode(index int) {
	if index < 0 || index >= len(m.config.Providers) {
		m.setStatus(statusWarning, m.tr("当前没有可编辑的供应商。", "There is no provider to edit."))
		return
	}

	m.blurPromptInput()
	m.mode = addMode
	m.editingIndex = index
	m.preloadForm(m.config.Providers[index])
	m.setStatus(statusInfo, m.tr("编辑供应商：修改基础 URL、API 密钥和模型；在基础 URL 或 API 密钥上按回车更新，Esc 取消。", "Edit the selected provider. Update the base URL, API key, and models; press Enter on Base URL or API Key to save, or Esc to cancel."))
}

func (m *appModel) cancelFormMode() {
	wasEditing := m.isEditingProvider()
	m.mode = listMode
	m.resetForm()
	if wasEditing {
		m.setStatus(statusInfo, m.tr("已取消编辑供应商。", "Canceled editing provider."))
		return
	}
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
	return runChecksCmd(selected, m.promptInput.Value(), m.concurrency)
}

func (m *appModel) focusPromptInput() {
	m.promptEditing = true
	m.promptInput.Focus()
}

func (m *appModel) blurPromptInput() {
	m.promptEditing = false
	m.promptInput.Blur()
}

func (m *appModel) buildProviderFromInputs() (provider, error) {
	baseURL, err := normalizeBaseURL(m.baseURLInput.Value())
	if err != nil {
		return provider{}, fmt.Errorf(m.tr("URL 无效：%v", "Invalid URL: %v"), err)
	}
	models := splitModels(m.modelsInput.Value())
	if len(models) == 0 {
		return provider{}, errors.New(m.tr("至少填写一个模型。", "At least one model is required."))
	}

	return provider{
		BaseURL: baseURL,
		APIKey:  strings.TrimSpace(m.apiKeyInput.Value()),
		Models:  models,
	}, nil
}

func (m appModel) formSaveSuccessStatus(provider provider) string {
	if m.isEditingProvider() {
		return fmt.Sprintf(m.tr("已更新供应商 %s，共 %d 个模型。", "Updated provider %s with %d model(s)."), provider.BaseURL, len(provider.Models))
	}
	return fmt.Sprintf(m.tr("已保存供应商 %s，共 %d 个模型。", "Saved provider %s with %d model(s)."), provider.BaseURL, len(provider.Models))
}

func (m appModel) formSaveFailureStatus(err error) string {
	if m.isEditingProvider() {
		return fmt.Sprintf(m.tr("更新供应商失败：%v", "Update provider failed: %v"), err)
	}
	return fmt.Sprintf(m.tr("保存配置失败：%v", "Save config failed: %v"), err)
}

func (m *appModel) saveProvider(provider provider) (int, error) {
	if m.isEditingProvider() {
		previous := m.config.Providers[m.editingIndex]
		m.config.Providers[m.editingIndex] = provider
		if err := saveConfig(m.configPath, m.config); err != nil {
			m.config.Providers[m.editingIndex] = previous
			return m.editingIndex, err
		}
		return m.editingIndex, nil
	}

	m.config.Providers = append(m.config.Providers, provider)
	if err := saveConfig(m.configPath, m.config); err != nil {
		m.config.Providers = m.config.Providers[:len(m.config.Providers)-1]
		return noEditingProviderIndex, err
	}
	return len(m.config.Providers) - 1, nil
}
