package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m appModel) Init() tea.Cmd {
	return tea.EnableMouseCellMotion
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyPlaceholders()
		m.syncFormPaneScroll()
		m.syncProviderPaneScroll(paneScrollDirectionNeutral)
		m.syncResultsPaneScroll()
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
	case tea.MouseMsg:
		if m.mode == listMode {
			return m.handleListMouse(msg)
		}
		return m, nil
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}

		if key == "ctrl+l" || (key == "l" && m.mode == listMode) {
			m.toggleLanguage()
			return m, nil
		}

		if m.mode != listMode {
			return m.handleFormKeys(msg)
		}
		return m.handleListKeys(msg)
	}

	if m.mode != listMode {
		return m.updateInputs(msg)
	}

	return m, nil
}

func (m appModel) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	direction := paneScrollDirectionNeutral

	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "p":
		m.focusListPane(providerPaneFocus)
		return m, nil
	case "r":
		m.focusListPane(resultsPaneFocus)
		return m, nil
	case "up", "k", "w", "down", "j", "s", "a", "e", "o", "enter":
		if m.running {
			m.setStatus(statusWarning, m.tr("检测进行中，请等待完成后再切换或操作。", "Checks are still running. Wait for completion before changing providers."))
			return m, nil
		}
	}

	switch msg.String() {
	case "up", "k", "w":
		if m.cursor > 0 {
			m.cursor--
			direction = paneScrollDirectionUp
		}
	case "down", "j", "s":
		if m.cursor < len(m.config.Providers)-1 {
			m.cursor++
			direction = paneScrollDirectionDown
		}
	case "a":
		m.enterAddMode()
	case "e":
		if len(m.config.Providers) == 0 {
			m.setStatus(statusWarning, m.tr("还没有可编辑的供应商，请先新增。", "No providers to edit yet. Add one first."))
			return m, nil
		}
		m.enterEditMode(m.cursor)
	case "o":
		m.enterRequestSettingsMode()
	case "enter":
		if len(m.config.Providers) == 0 {
			m.setStatus(statusWarning, m.tr("请先新增至少一个供应商后再检测。", "Add at least one provider before running checks."))
			return m, nil
		}
		return m, tea.Batch(m.spinner.Tick, m.startChecks())
	}

	m.syncProviderPaneScroll(direction)
	return m, nil
}

func (m appModel) handleListMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch mouseEventKind(msg) {
	case "left press", "left release":
		providerBounds, resultsBounds := m.listPaneBounds()
		if providerBounds.contains(msg.X, msg.Y) {
			m.focusListPane(providerPaneFocus)
		} else if resultsBounds.contains(msg.X, msg.Y) {
			m.focusListPane(resultsPaneFocus)
		}
	case "wheel up":
		m.scrollFocusedListPane(-1)
	case "wheel down":
		m.scrollFocusedListPane(1)
	}

	return m, nil
}

func (m appModel) handleFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cancelFormMode()
		return m, nil
	case "up", "down":
		return m.handleVerticalFormNavigation(msg)
	case "tab", "shift+tab":
		m.cycleFocus(msg.String())
		return m, nil
	case "ctrl+s":
		if m.mode == requestSettingsMode {
			return m.submitRequestSettingsForm()
		}
		return m.submitProviderForm()
	case "left", "right":
		if m.mode == requestSettingsMode && m.focusIndex == requestSettingsModeField {
			m.toggleRequestMode()
			return m, nil
		}
	case "enter":
		if m.mode == requestSettingsMode && m.focusIndex == requestSettingsModeField {
			m.toggleRequestMode()
			return m, nil
		}
		return m.updateInputs(msg)
	}

	return m.updateInputs(msg)
}

func (m appModel) submitProviderForm() (tea.Model, tea.Cmd) {
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
	m.syncProviderPaneScroll(paneScrollDirectionNeutral)
	m.setStatus(statusSuccess, successStatus)
	return m, nil
}

func (m *appModel) finishRun(results []modelResult) {
	m.running = false
	m.results = results
	m.activeResult = maxInt(0, len(results)-1)
	m.syncResultsPaneScroll()
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
	m.syncProviderPaneScroll(paneScrollDirectionNeutral)
	m.setStatus(statusInfo, m.tr("新增供应商：模型支持逗号或换行；按 Ctrl+S 保存，Esc 取消。", "Add a provider. Models accept commas or new lines; press Ctrl+S to save, or Esc to cancel."))
}

func (m *appModel) enterEditMode(index int) {
	if index < 0 || index >= len(m.config.Providers) {
		m.setStatus(statusWarning, m.tr("当前没有可编辑的供应商。", "There is no provider to edit."))
		return
	}

	m.mode = addMode
	m.editingIndex = index
	m.preloadForm(m.config.Providers[index])
	m.syncProviderPaneScroll(paneScrollDirectionNeutral)
	m.setStatus(statusInfo, m.tr("编辑供应商：修改基础 URL、API 密钥和模型；按 Ctrl+S 更新，Esc 取消。", "Edit the selected provider. Update the base URL, API key, and models; press Ctrl+S to update, or Esc to cancel."))
}

func (m *appModel) cancelFormMode() {
	wasEditing := m.isEditingProvider()
	wasRequestSettings := m.mode == requestSettingsMode
	m.mode = listMode
	m.resetForm()
	m.syncProviderPaneScroll(paneScrollDirectionNeutral)
	if wasEditing {
		m.setStatus(statusInfo, m.tr("已取消编辑供应商。", "Canceled editing provider."))
		return
	}
	if wasRequestSettings {
		m.preloadRequestSettingsForm(m.config.RequestSettings)
		m.setStatus(statusInfo, m.tr("已取消请求设置编辑。", "Canceled request settings edit."))
		return
	}
	m.setStatus(statusInfo, m.tr("已取消新增供应商。", "Canceled adding provider."))
}

func (m *appModel) enterRequestSettingsMode() {
	m.mode = requestSettingsMode
	m.editingIndex = noEditingProviderIndex
	m.preloadRequestSettingsForm(m.config.RequestSettings)
	m.syncProviderPaneScroll(paneScrollDirectionNeutral)
	m.setStatus(statusInfo, m.tr("请求设置：修改提示词、超时、请求方式与重试次数；按 Ctrl+S 保存，Esc 取消。", "Request settings: update the prompt, timeout, request mode, and retries; press Ctrl+S to save, or Esc to cancel."))
}

func (m appModel) submitRequestSettingsForm() (tea.Model, tea.Cmd) {
	settings, err := m.buildRequestSettingsFromInputs()
	if err != nil {
		m.setStatus(statusError, err.Error())
		return m, nil
	}
	previous := m.config.RequestSettings
	m.config.RequestSettings = settings
	if err := saveConfig(m.configPath, m.config); err != nil {
		m.config.RequestSettings = previous
		m.setStatus(statusError, fmt.Sprintf(m.tr("保存请求设置失败：%v", "Save request settings failed: %v"), err))
		return m, nil
	}
	m.mode = listMode
	m.preloadRequestSettingsForm(settings)
	m.syncProviderPaneScroll(paneScrollDirectionNeutral)
	m.setStatus(statusSuccess, m.tr("已保存请求设置。", "Saved request settings."))
	return m, nil
}

func (m *appModel) toggleRequestMode() {
	if m.requestMode == requestModeResponses {
		m.requestMode = requestModeCompatible
	} else {
		m.requestMode = requestModeResponses
	}
	if m.mode == requestSettingsMode {
		m.syncFormPaneScroll()
	}
}

func (m *appModel) startChecks() tea.Cmd {
	selected := m.config.Providers[m.cursor]
	m.running = true
	m.results = nil
	m.activeResult = 0
	m.resultsPaneScrollOffset = 0
	if m.lang == langEN {
		m.setStatus(statusLoading, fmt.Sprintf("Checking %d model(s) from %s with concurrency %d...", len(selected.Models), selected.BaseURL, m.concurrency))
	} else {
		m.setStatus(statusLoading, fmt.Sprintf("正在检测 %s 的 %d 个模型（并发 %d）...", selected.BaseURL, len(selected.Models), m.concurrency))
	}
	return runChecksCmd(selected, normalizeRequestSettings(m.config.RequestSettings), m.concurrency)
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

func (m *appModel) buildRequestSettingsFromInputs() (requestSettings, error) {
	prompt := strings.TrimSpace(m.requestPromptInput.Value())
	if prompt == "" {
		return requestSettings{}, errors.New(m.tr("请求提示词不能为空。", "Request prompt cannot be empty."))
	}

	timeoutSeconds, err := parsePositiveInt(m.requestTimeoutInput.Value())
	if err != nil {
		return requestSettings{}, fmt.Errorf(m.tr("超时时间无效：%v", "Invalid timeout: %v"), err)
	}

	retryCount, err := parseNonNegativeInt(m.requestRetryInput.Value())
	if err != nil {
		return requestSettings{}, fmt.Errorf(m.tr("重试次数无效：%v", "Invalid retry count: %v"), err)
	}

	return normalizeRequestSettings(requestSettings{
		Prompt:         prompt,
		TimeoutSeconds: timeoutSeconds,
		Mode:           m.requestMode,
		RetryCount:     retryCount,
	}), nil
}

func parsePositiveInt(raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, errors.New("must be a whole number")
	}
	if value <= 0 {
		return 0, errors.New("must be greater than 0")
	}
	return value, nil
}

func parseNonNegativeInt(raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, errors.New("must be a whole number")
	}
	if value < 0 {
		return 0, errors.New("must be 0 or greater")
	}
	return value, nil
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
