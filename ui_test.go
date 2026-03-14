package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}

func TestFormViewWithChinesePlaceholdersDoesNotPanic(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 120
	m.applyPlaceholders()

	assertNoPanic(t, func() {
		_ = m.formView()
	})
}

func TestLanguageToggleInAddModeDoesNotPanic(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 120
	m.applyPlaceholders()

	m.toggleLanguage()
	assertNoPanic(t, func() {
		_ = m.formView()
	})

	m.toggleLanguage()
	assertNoPanic(t, func() {
		_ = m.formView()
	})
}

func TestApplyInputLocaleUsesSharedMetadataAndSizing(t *testing.T) {
	inputs := newInputs(langEN)
	defaultPaneWidth := formPaneWidth(0)
	defaultInputWidth := inputWidthForFormPane(defaultPaneWidth)
	for i, field := range formFields {
		if inputs[i].Width != defaultInputWidth {
			t.Fatalf("expected default input width %d, got %d at index %d", defaultInputWidth, inputs[i].Width, i)
		}
		if inputs[i].Placeholder != safePlaceholder(field.placeholder.forLang(langEN)) {
			t.Fatalf("unexpected English placeholder at index %d: %q", i, inputs[i].Placeholder)
		}
	}

	widePaneWidth := formPaneWidth(120)
	applyInputLocale(inputs, langZH, widePaneWidth)
	wideInputWidth := inputWidthForFormPane(widePaneWidth)
	for i, field := range formFields {
		if inputs[i].Width != wideInputWidth {
			t.Fatalf("expected Chinese input width %d, got %d at index %d", wideInputWidth, inputs[i].Width, i)
		}
		if inputs[i].Placeholder != safePlaceholder(field.placeholder.forLang(langZH)) {
			t.Fatalf("unexpected Chinese placeholder at index %d: %q", i, inputs[i].Placeholder)
		}
		if inputs[i].Placeholder != "" {
			t.Fatalf("expected Chinese placeholder to be suppressed for safety, got %q at index %d", inputs[i].Placeholder, i)
		}
	}
}

func TestWindowSizeMsgUpdatesInputWidths(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	wantWidth := inputWidthForFormPane(formPaneWidth(90))
	for i := range got.inputs {
		if got.inputs[i].Width != wantWidth {
			t.Fatalf("expected input width %d, got %d at index %d", wantWidth, got.inputs[i].Width, i)
		}
	}
}

func TestModelsInputHasNoCharLimit(t *testing.T) {
	inputs := newInputs(langEN)

	if inputs[0].CharLimit != defaultInputCharLimit {
		t.Fatalf("expected Base URL char limit %d, got %d", defaultInputCharLimit, inputs[0].CharLimit)
	}
	if inputs[1].CharLimit != defaultInputCharLimit {
		t.Fatalf("expected API key char limit %d, got %d", defaultInputCharLimit, inputs[1].CharLimit)
	}
	if inputs[2].CharLimit != 0 {
		t.Fatalf("expected Models char limit 0 (unlimited), got %d", inputs[2].CharLimit)
	}
}

func TestModelEnterWithNoProvidersSetsStatusAndNoCommand(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.status != "请先新增至少一个供应商后再检测。" {
		t.Fatalf("unexpected status: %q", got.status)
	}
	if got.statusKind != statusWarning {
		t.Fatalf("expected warning status kind, got %v", got.statusKind)
	}
}

func TestModelEnterStartsChecksAndClearsResults(t *testing.T) {
	m := newModel(appConfig{Providers: []provider{{BaseURL: "https://example.com", Models: []string{"gpt-4o-mini"}}}}, "juicy-providers.json")
	m.results = []modelResult{{Model: "old", Value: "1"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(appModel)

	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if !got.running {
		t.Fatal("expected running to be true")
	}
	if got.results != nil {
		t.Fatalf("expected previous results cleared, got %+v", got.results)
	}
	if got.status != "正在检测 https://example.com 的 1 个模型（并发 5）..." {
		t.Fatalf("unexpected status: %q", got.status)
	}
	if got.statusKind != statusLoading {
		t.Fatalf("expected loading status kind, got %v", got.statusKind)
	}
}

func TestModelRunFinishedMsgUpdatesStatus(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		m := newModel(appConfig{}, "juicy-providers.json")
		m.running = true

		updated, cmd := m.Update(runFinishedMsg{Results: nil})
		got := updated.(appModel)

		if cmd != nil {
			t.Fatal("expected no command")
		}
		if got.running {
			t.Fatal("expected running to be false")
		}
		if got.status != "当前供应商没有可检测模型。" {
			t.Fatalf("unexpected status: %q", got.status)
		}
		if got.statusKind != statusWarning {
			t.Fatalf("expected warning status kind, got %v", got.statusKind)
		}
	})

	t.Run("all success", func(t *testing.T) {
		m := newModel(appConfig{}, "juicy-providers.json")

		updated, _ := m.Update(runFinishedMsg{Results: []modelResult{{Model: "a", Value: "1"}, {Model: "b", Value: "2"}}})
		got := updated.(appModel)

		if got.status != "已完成 2 个模型检测。" {
			t.Fatalf("unexpected status: %q", got.status)
		}
		if got.statusKind != statusSuccess {
			t.Fatalf("expected success status kind, got %v", got.statusKind)
		}
	})

	t.Run("partial failure", func(t *testing.T) {
		m := newModel(appConfig{}, "juicy-providers.json")

		updated, _ := m.Update(runFinishedMsg{Results: []modelResult{{Model: "a", Value: "1"}, {Model: "b", Error: "boom"}}})
		got := updated.(appModel)

		if got.status != "检测完成，错误 1/2。" {
			t.Fatalf("unexpected status: %q", got.status)
		}
		if got.statusKind != statusWarning {
			t.Fatalf("expected warning status kind, got %v", got.statusKind)
		}
	})
}

func TestModelAddProviderSaveFailureRollsBackProviderAppend(t *testing.T) {
	configDir := t.TempDir()
	m := newModel(appConfig{}, configDir)
	m.mode = addMode
	m.inputs[0].SetValue("https://example.com/v1")
	m.inputs[1].SetValue("secret")
	m.inputs[2].SetValue("gpt-4o-mini")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if len(got.config.Providers) != 0 {
		t.Fatalf("expected provider append rollback, got %d providers", len(got.config.Providers))
	}
	if got.mode != addMode {
		t.Fatalf("expected to remain in add mode, got %v", got.mode)
	}
	if !strings.HasPrefix(got.status, "保存配置失败：") {
		t.Fatalf("unexpected status: %q", got.status)
	}
	if got.statusKind != statusError {
		t.Fatalf("expected error status kind, got %v", got.statusKind)
	}
}

func TestModelRunningBlocksListActions(t *testing.T) {
	m := newModel(appConfig{Providers: []provider{{BaseURL: "https://one", Models: []string{"a"}}, {BaseURL: "https://two", Models: []string{"b"}}}}, "juicy-providers.json")
	m.cursor = 1
	m.running = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.cursor != 1 {
		t.Fatalf("expected cursor unchanged, got %d", got.cursor)
	}
	if got.status != "检测进行中，请等待完成后再切换或操作。" {
		t.Fatalf("unexpected status: %q", got.status)
	}
	if got.statusKind != statusWarning {
		t.Fatalf("expected warning status kind, got %v", got.statusKind)
	}
}

func TestModelToggleLanguageUpdatesStatusAndPlaceholders(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode

	updated, cmd := m.Update(keyRunes('l'))
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.lang != langEN {
		t.Fatalf("expected English language, got %v", got.lang)
	}
	if got.status != "Language switched to English." {
		t.Fatalf("unexpected status: %q", got.status)
	}
	if got.statusKind != statusInfo {
		t.Fatalf("expected info status kind, got %v", got.statusKind)
	}
	if got.inputs[0].Placeholder == "" {
		t.Fatal("expected English placeholder to be visible")
	}
}

func TestModelAddModeEscCancelsAndReturnsToList(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != listMode {
		t.Fatalf("expected list mode, got %v", got.mode)
	}
	if got.status != "已取消新增供应商。" {
		t.Fatalf("unexpected status: %q", got.status)
	}
	if got.statusKind != statusInfo {
		t.Fatalf("expected info status kind, got %v", got.statusKind)
	}
}

func TestModelTabNavigationCyclesFocus(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(appModel)
	if got.focusIndex != 1 {
		t.Fatalf("expected focus index 1, got %d", got.focusIndex)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = updated.(appModel)
	if got.focusIndex != 2 {
		t.Fatalf("expected focus index 2, got %d", got.focusIndex)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	got = updated.(appModel)
	if got.focusIndex != 1 {
		t.Fatalf("expected focus index 1 after shift+tab, got %d", got.focusIndex)
	}
}

func TestModelRejectsInvalidURLAndEmptyModels(t *testing.T) {
	t.Run("invalid URL", func(t *testing.T) {
		m := newModel(appConfig{}, "juicy-providers.json")
		m.mode = addMode
		m.inputs[0].SetValue("example.com")
		m.inputs[2].SetValue("gpt-4o-mini")

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		got := updated.(appModel)

		if got.mode != addMode {
			t.Fatalf("expected add mode, got %v", got.mode)
		}
		if !strings.HasPrefix(got.status, "URL 无效：") {
			t.Fatalf("unexpected status: %q", got.status)
		}
		if got.statusKind != statusError {
			t.Fatalf("expected error status kind, got %v", got.statusKind)
		}
	})

	t.Run("empty models", func(t *testing.T) {
		m := newModel(appConfig{}, "juicy-providers.json")
		m.mode = addMode
		m.inputs[0].SetValue("https://example.com/v1")

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		got := updated.(appModel)

		if got.mode != addMode {
			t.Fatalf("expected add mode, got %v", got.mode)
		}
		if got.status != "至少填写一个模型。" {
			t.Fatalf("unexpected status: %q", got.status)
		}
		if got.statusKind != statusError {
			t.Fatalf("expected error status kind, got %v", got.statusKind)
		}
	})
}

func TestModelSavesProviderAndMovesCursor(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "providers.json")
	m := newModel(appConfig{}, configPath)
	m.mode = addMode
	m.inputs[0].SetValue("https://example.com/v1/")
	m.inputs[1].SetValue("secret")
	m.inputs[2].SetValue("gpt-4o-mini, qwen-max")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != listMode {
		t.Fatalf("expected list mode, got %v", got.mode)
	}
	if got.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", got.cursor)
	}
	if len(got.config.Providers) != 1 {
		t.Fatalf("expected one provider, got %d", len(got.config.Providers))
	}
	if got.config.Providers[0].BaseURL != "https://example.com/v1" {
		t.Fatalf("unexpected base URL: %q", got.config.Providers[0].BaseURL)
	}
	if got.status != "已保存供应商 https://example.com/v1，共 2 个模型。" {
		t.Fatalf("unexpected status: %q", got.status)
	}
	if got.statusKind != statusSuccess {
		t.Fatalf("expected success status kind, got %v", got.statusKind)
	}
	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if len(loaded.Providers) != 1 {
		t.Fatalf("expected saved provider, got %d", len(loaded.Providers))
	}
}

func TestStatusLineUsesSeverityStyles(t *testing.T) {
	tests := []struct {
		name string
		kind statusKind
		text string
		want string
	}{
		{name: "info", kind: statusInfo, text: "info", want: infoStyle.Render("info")},
		{name: "success", kind: statusSuccess, text: "ok", want: successStyle.Render("ok")},
		{name: "error", kind: statusError, text: "boom", want: errorStyle.Render("boom")},
		{name: "warning", kind: statusWarning, text: "warn", want: warningStyle.Render("warn")},
		{name: "loading", kind: statusLoading, text: "wait", want: loadingStyle.Render("wait")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(appConfig{}, "juicy-providers.json")
			m.setStatus(tc.kind, tc.text)
			if got := m.statusLine(); got != tc.want {
				t.Fatalf("unexpected status line: %q", got)
			}
		})
	}

	m := newModel(appConfig{}, "juicy-providers.json")
	m.status = ""
	m.statusKind = statusInfo
	if got := m.statusLine(); got != infoStyle.Render("就绪") {
		t.Fatalf("unexpected ready status line: %q", got)
	}
}

func TestListViewUsesDistinctSemanticStyles(t *testing.T) {
	m := newModel(appConfig{Providers: []provider{{BaseURL: "https://one", Models: []string{"a", "b"}}}}, "juicy-providers.json")
	m.cursor = 0
	providerView := m.providerListView()
	selectedLine := selectionStyle.Render("https://one（2 个模型）")
	if !strings.Contains(providerView, selectedLine) {
		t.Fatalf("expected selected provider line in view: %q", providerView)
	}

	m.running = true
	loadingView := m.resultListView()
	loadingLine := loadingStyle.Render(m.spinner.View() + " 正在执行检测...")
	if !strings.Contains(loadingView, loadingLine) {
		t.Fatalf("expected loading line in view: %q", loadingView)
	}

	m.running = false
	m.results = []modelResult{{Model: "ok", Value: "7"}, {Model: "bad", Error: "boom"}}
	resultView := m.resultListView()
	if !strings.Contains(resultView, successStyle.Render("ok -> 7")) {
		t.Fatalf("expected success result in view: %q", resultView)
	}
	if !strings.Contains(resultView, errorStyle.Render("bad -> boom")) {
		t.Fatalf("expected error result in view: %q", resultView)
	}
}

func TestListViewUsesSharedHeadersAndEmptyStates(t *testing.T) {
	for _, width := range []int{80, 140} {
		t.Run(fmt.Sprintf("width-%d", width), func(t *testing.T) {
			m := newModel(appConfig{}, "juicy-providers.json")
			m.width = width
			view := m.listView()

			if !strings.Contains(view, pageTitleStyle.Render("Juicy 批量检测器")) {
				t.Fatalf("expected page title in view: %q", view)
			}
			if !strings.Contains(view, renderSectionHeader("供应商")) {
				t.Fatalf("expected provider section header in view: %q", view)
			}
			if !strings.Contains(view, renderSectionHeader("结果")) {
				t.Fatalf("expected result section header in view: %q", view)
			}
			if !strings.Contains(view, renderEmptyState("还没有保存任何供应商，按 'a' 新增。")) {
				t.Fatalf("expected provider empty state in view: %q", view)
			}
			if !strings.Contains(view, "暂无结果，请先选择供应商并按") || !strings.Contains(view, "Enter。") {
				t.Fatalf("expected result empty state in view: %q", view)
			}
			if !strings.Contains(view, renderShortcutFooter("快捷键：a 新增供应商 | Enter 开始检测 | j/k 移动 | l 切换中英 | q 退出")) {
				t.Fatalf("expected shortcut footer in view: %q", view)
			}
		})
	}
}

func TestFormViewShowsFieldGuidanceInBothLanguages(t *testing.T) {
	tests := []struct {
		name  string
		lang  appLanguage
		width int
		title string
		intro string
		keys  string
	}{
		{
			name:  "zh-narrow",
			lang:  langZH,
			width: 80,
			title: "新增供应商",
			intro: "请填写 OAI 兼容 base URL、API key 和模型列表（逗号分隔）。",
			keys:  "快捷键：tab/shift+tab 切换焦点 | Enter 保存 | Esc 取消 | l 切换中英",
		},
		{
			name:  "zh-wide",
			lang:  langZH,
			width: 140,
			title: "新增供应商",
			intro: "请填写 OAI 兼容 base URL、API key 和模型列表（逗号分隔）。",
			keys:  "快捷键：tab/shift+tab 切换焦点 | Enter 保存 | Esc 取消 | l 切换中英",
		},
		{
			name:  "en-narrow",
			lang:  langEN,
			width: 80,
			title: "Add Provider",
			intro: "Fill in an OAI-compatible base URL, API key, and comma-separated models.",
			keys:  "Keys: tab/shift+tab move | enter save | esc cancel | l toggle lang",
		},
		{
			name:  "en-wide",
			lang:  langEN,
			width: 140,
			title: "Add Provider",
			intro: "Fill in an OAI-compatible base URL, API key, and comma-separated models.",
			keys:  "Keys: tab/shift+tab move | enter save | esc cancel | l toggle lang",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(appConfig{}, "juicy-providers.json")
			m.mode = addMode
			m.lang = tc.lang
			m.width = tc.width
			m.applyPlaceholders()

			view := m.formView()

			if !strings.Contains(view, pageTitleStyle.Render(tc.title)) {
				t.Fatalf("expected page title in form view: %q", view)
			}
			if !strings.Contains(view, helperTextStyle.Render(tc.intro)) {
				t.Fatalf("expected intro copy in form view: %q", view)
			}
			if !strings.Contains(view, renderShortcutFooter(tc.keys)) {
				t.Fatalf("expected footer in form view: %q", view)
			}
			for i, field := range formFields {
				if !strings.Contains(view, renderFieldLabel(field.label.forLang(tc.lang))) {
					t.Fatalf("expected field label %q in view: %q", field.label.forLang(tc.lang), view)
				}
				if !strings.Contains(view, helperTextStyle.Render(field.helper.forLang(tc.lang))) {
					t.Fatalf("expected field helper %q in view: %q", field.helper.forLang(tc.lang), view)
				}
				if tc.lang == langZH && m.inputs[i].Placeholder != "" {
					t.Fatalf("expected Chinese placeholder suppressed at index %d, got %q", i, m.inputs[i].Placeholder)
				}
				if tc.lang == langEN && m.inputs[i].Placeholder == "" {
					t.Fatalf("expected English placeholder visible at index %d", i)
				}
			}
		})
	}
}

func keyRunes(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}
