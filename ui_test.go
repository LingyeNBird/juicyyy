package main

import (
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

	assertNoPanic(t, func() {
		_ = m.formView()
	})
}

func TestLanguageToggleInAddModeDoesNotPanic(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 120

	m.toggleLanguage()
	assertNoPanic(t, func() {
		_ = m.formView()
	})

	m.toggleLanguage()
	assertNoPanic(t, func() {
		_ = m.formView()
	})
}

func TestApplyInputLocaleAdjustsWidthForWidePlaceholders(t *testing.T) {
	inputs := newInputs(langEN)
	for i := range inputs {
		if inputs[i].Width != defaultInputWidth {
			t.Fatalf("expected English input width %d, got %d at index %d", defaultInputWidth, inputs[i].Width, i)
		}
	}

	applyInputLocale(inputs, langZH)
	for i := range inputs {
		if inputs[i].Width != defaultInputWidth {
			t.Fatalf("expected Chinese input width %d, got %d at index %d", defaultInputWidth, inputs[i].Width, i)
		}
		if inputs[i].Placeholder != "" {
			t.Fatalf("expected Chinese placeholder to be suppressed for safety, got %q at index %d", inputs[i].Placeholder, i)
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
	})

	t.Run("all success", func(t *testing.T) {
		m := newModel(appConfig{}, "juicy-providers.json")

		updated, _ := m.Update(runFinishedMsg{Results: []modelResult{{Model: "a", Value: "1"}, {Model: "b", Value: "2"}}})
		got := updated.(appModel)

		if got.status != "已完成 2 个模型检测。" {
			t.Fatalf("unexpected status: %q", got.status)
		}
	})

	t.Run("partial failure", func(t *testing.T) {
		m := newModel(appConfig{}, "juicy-providers.json")

		updated, _ := m.Update(runFinishedMsg{Results: []modelResult{{Model: "a", Value: "1"}, {Model: "b", Error: "boom"}}})
		got := updated.(appModel)

		if got.status != "检测完成，错误 1/2。" {
			t.Fatalf("unexpected status: %q", got.status)
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
	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if len(loaded.Providers) != 1 {
		t.Fatalf("expected saved provider, got %d", len(loaded.Providers))
	}
}

func keyRunes(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}
