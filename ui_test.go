package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}

func assertContainsAll(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Fatalf("expected %q in view: %q", part, text)
		}
	}
}

func compactForContains(text string) string {
	replacer := strings.NewReplacer(
		" ", "",
		"\n", "",
		"\t", "",
		"│", "",
	)
	return replacer.Replace(text)
}

func stripANSI(text string) string {
	return ansiEscapePattern.ReplaceAllString(text, "")
}

func renderedLines(text string) []string {
	trimmed := strings.TrimRight(stripANSI(text), "\n")
	if trimmed == "" {
		return []string{""}
	}

	lines := strings.Split(trimmed, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return lines
}

func firstRuneString(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[0])
}

func lastRuneString(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[len(runes)-1])
}

func trimLastRune(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}
	return string(runes[:len(runes)-1])
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
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.lang = langEN
	m.applyPlaceholders()

	defaultPaneWidth := listPaneWidth(0)
	defaultInputWidth := inputWidthForFormPane(defaultPaneWidth)
	if m.baseURLInput.Width != defaultInputWidth {
		t.Fatalf("expected default Base URL width %d, got %d", defaultInputWidth, m.baseURLInput.Width)
	}
	if m.apiKeyInput.Width != defaultInputWidth {
		t.Fatalf("expected default API key width %d, got %d", defaultInputWidth, m.apiKeyInput.Width)
	}
	if m.baseURLInput.Placeholder != safePlaceholder(formFields[addProviderBaseURLField].placeholder.forLang(langEN)) {
		t.Fatalf("unexpected English Base URL placeholder: %q", m.baseURLInput.Placeholder)
	}
	if m.apiKeyInput.Placeholder != safePlaceholder(formFields[addProviderAPIKeyField].placeholder.forLang(langEN)) {
		t.Fatalf("unexpected English API key placeholder: %q", m.apiKeyInput.Placeholder)
	}
	if m.modelsInput.Placeholder != safePlaceholder(formFields[addProviderModelsField].placeholder.forLang(langEN)) {
		t.Fatalf("unexpected English models placeholder: %q", m.modelsInput.Placeholder)
	}
	if m.modelsInput.Width() != modelsInputTextWidthForPane(defaultPaneWidth) {
		t.Fatalf("expected default Models text width %d, got %d", modelsInputTextWidthForPane(defaultPaneWidth), m.modelsInput.Width())
	}
	if m.modelsInput.Height() != 1 {
		t.Fatalf("expected compact default Models height 1, got %d", m.modelsInput.Height())
	}

	widePaneWidth := formPaneWidth(120)
	applyFormLocale(&m.baseURLInput, &m.apiKeyInput, &m.modelsInput, langZH, widePaneWidth)
	wideInputWidth := inputWidthForFormPane(widePaneWidth)
	if m.baseURLInput.Width != wideInputWidth {
		t.Fatalf("expected Chinese Base URL width %d, got %d", wideInputWidth, m.baseURLInput.Width)
	}
	if m.apiKeyInput.Width != wideInputWidth {
		t.Fatalf("expected Chinese API key width %d, got %d", wideInputWidth, m.apiKeyInput.Width)
	}
	if m.baseURLInput.Placeholder != "" {
		t.Fatalf("expected Chinese Base URL placeholder suppressed, got %q", m.baseURLInput.Placeholder)
	}
	if m.apiKeyInput.Placeholder != "" {
		t.Fatalf("expected Chinese API key placeholder suppressed, got %q", m.apiKeyInput.Placeholder)
	}
	if m.modelsInput.Placeholder != "" {
		t.Fatalf("expected Chinese models placeholder suppressed, got %q", m.modelsInput.Placeholder)
	}
	if m.modelsInput.Width() != modelsInputTextWidthForPane(widePaneWidth) {
		t.Fatalf("expected Chinese Models text width %d, got %d", modelsInputTextWidthForPane(widePaneWidth), m.modelsInput.Width())
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
	wantWidth := inputWidthForFormPane(listPaneWidth(90))
	if got.baseURLInput.Width != wantWidth {
		t.Fatalf("expected Base URL width %d, got %d", wantWidth, got.baseURLInput.Width)
	}
	if got.apiKeyInput.Width != wantWidth {
		t.Fatalf("expected API key width %d, got %d", wantWidth, got.apiKeyInput.Width)
	}
	if got.modelsInput.Placeholder != "" {
		t.Fatalf("expected Chinese models placeholder suppressed after resize, got %q", got.modelsInput.Placeholder)
	}
	if got.modelsInput.Width() != modelsInputTextWidthForPane(listPaneWidth(90)) {
		t.Fatalf("expected Models text width %d, got %d", modelsInputTextWidthForPane(listPaneWidth(90)), got.modelsInput.Width())
	}
	if got.modelsInput.Height() != 1 {
		t.Fatalf("expected compact Models height 1 after resize, got %d", got.modelsInput.Height())
	}
}

func TestWindowSizeMsgReclampsFormPaneScrollToKeepCursorVisible(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 100
	m.height = 18
	m.modelsInput.SetValue(strings.Join([]string{
		"model-01", "model-02", "model-03", "model-04", "model-05", "model-06", "model-07", "model-08",
	}, "\n"))
	setFocusedFormField(&m, addProviderModelsField)
	m.syncFormPaneScroll()

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 14})
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.formPaneScrollOffset <= 0 {
		t.Fatalf("expected resize to keep a positive form scroll offset, got %d", got.formPaneScrollOffset)
	}
	assertActiveFormCursorVisible(t, got)
}

func TestModelsInputUsesProviderLikePromptAndCompactHeight(t *testing.T) {
	baseURLInput, apiKeyInput, modelsInput := newProviderInputs(langEN)

	if baseURLInput.CharLimit != defaultInputCharLimit {
		t.Fatalf("expected Base URL char limit %d, got %d", defaultInputCharLimit, baseURLInput.CharLimit)
	}
	if apiKeyInput.CharLimit != defaultInputCharLimit {
		t.Fatalf("expected API key char limit %d, got %d", defaultInputCharLimit, apiKeyInput.CharLimit)
	}
	if modelsInput.CharLimit != 0 {
		t.Fatalf("expected Models char limit 0 (unlimited), got %d", modelsInput.CharLimit)
	}
	if modelsInput.Prompt != modelsInputPrompt {
		t.Fatalf("expected Models prompt %q, got %q", modelsInputPrompt, modelsInput.Prompt)
	}
	if modelsInput.ShowLineNumbers {
		t.Fatal("expected Models textarea line numbers disabled")
	}
	if modelsInput.Height() != 1 {
		t.Fatalf("expected compact Models height 1, got %d", modelsInput.Height())
	}

	lines := renderedLines(modelsInput.View())
	if len(lines) != 1 {
		t.Fatalf("expected single rendered row, got %d: %#v", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], modelsInputPrompt) {
		t.Fatalf("expected first rendered row to start with %q, got %q", modelsInputPrompt, lines[0])
	}
}

func TestModelsInputFocusedStyleRemovesCursorLineBackgroundHighlight(t *testing.T) {
	modelsInput := newModelsInput()
	modelsInput.Focus()

	if _, ok := modelsInput.FocusedStyle.CursorLine.GetBackground().(lipgloss.NoColor); !ok {
		t.Fatalf("expected focused cursor line background cleared, got %T", modelsInput.FocusedStyle.CursorLine.GetBackground())
	}
	if got, want := modelsInput.FocusedStyle.CursorLine.GetForeground(), inputStyle.GetForeground(); got != want {
		t.Fatalf("expected focused cursor line foreground %v, got %v", want, got)
	}
	if got, want := modelsInput.BlurredStyle.CursorLine.GetForeground(), inputStyle.GetForeground(); got != want {
		t.Fatalf("expected blurred cursor line foreground %v, got %v", want, got)
	}
}

func TestModelsInputMatchesPaneContentWidthInsteadOfNarrowFormWidth(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 120
	m.applyPlaceholders()

	paneWidth := listPaneWidth(m.width)
	wantOuterWidth := modelsInputWidthForPane(paneWidth)
	wantInnerWidth := modelsInputTextWidthForPane(paneWidth)
	narrowWidth := inputWidthForFormPane(paneWidth)

	if got := m.modelsInput.Width(); got != wantInnerWidth {
		t.Fatalf("expected Models inner width %d, got %d", wantInnerWidth, got)
	}
	if got := m.modelsInput.Width() + lipgloss.Width(modelsInputPrompt); got != wantOuterWidth {
		t.Fatalf("expected Models outer width %d, got %d", wantOuterWidth, got)
	}
	if wantOuterWidth <= narrowWidth {
		t.Fatalf("test setup invalid: pane content width %d should exceed narrowed width %d", wantOuterWidth, narrowWidth)
	}
}

func TestModelsInputWrappedContinuationRowsUseSpaceIndentAndDynamicHeight(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 80
	value := strings.Repeat("gpt-4o-mini-super-long-model-name-", 3)
	m.modelsInput.SetValue(value)
	syncModelsInputLayout(&m.modelsInput, listPaneWidth(m.width))
	m.modelsInput.Blur()

	wantHeight := modelsInputHeightForValue(value, listPaneWidth(m.width))
	if wantHeight <= 1 {
		t.Fatalf("test setup invalid: expected wrapped height > 1, got %d", wantHeight)
	}
	if m.modelsInput.Height() != wantHeight {
		t.Fatalf("expected Models height %d, got %d", wantHeight, m.modelsInput.Height())
	}

	lines := renderedLines(m.modelsInput.View())
	if len(lines) != wantHeight {
		t.Fatalf("expected %d rendered rows, got %d: %#v", wantHeight, len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], modelsInputPrompt) {
		t.Fatalf("expected first row to start with %q, got %q", modelsInputPrompt, lines[0])
	}
	for i := 1; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], modelsInputIndent) {
			t.Fatalf("expected continuation row %d to start with %q, got %q", i, modelsInputIndent, lines[i])
		}
		if strings.HasPrefix(lines[i], modelsInputPrompt) {
			t.Fatalf("expected continuation row %d not to repeat %q, got %q", i, modelsInputPrompt, lines[i])
		}
	}
}

func TestModelsInputMultilineValueStaysFullyVisible(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 100
	value := "gpt-4o-mini\nqwen-max, claude-3.5-sonnet\nllama-3.1-8b"
	m.modelsInput.SetValue(value)
	syncModelsInputLayout(&m.modelsInput, listPaneWidth(m.width))
	m.modelsInput.Blur()

	wantHeight := modelsInputHeightForValue(value, listPaneWidth(m.width))
	if m.modelsInput.Height() != wantHeight {
		t.Fatalf("expected Models height %d, got %d", wantHeight, m.modelsInput.Height())
	}

	view := stripANSI(m.modelsInput.View())
	if !strings.Contains(view, "gpt-4o-mini") || !strings.Contains(view, "qwen-max") || !strings.Contains(view, "claude-3.5-sonnet") || !strings.Contains(view, "llama-3.1-8b") {
		t.Fatalf("expected multiline Models view to keep all content visible, got %q", view)
	}
	if got := len(renderedLines(view)); got != wantHeight {
		t.Fatalf("expected %d visible rows, got %d", wantHeight, got)
	}
}

func TestModelsInputResizeResyncsWrappedHeight(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 120
	m.modelsInput.SetValue(strings.Repeat("qwen-max-compatible-super-long-name-", 3))
	m.applyPlaceholders()
	wideHeight := m.modelsInput.Height()
	wideWidth := m.modelsInput.Width()

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.modelsInput.Height() <= wideHeight {
		t.Fatalf("expected narrower resize to increase height beyond %d, got %d", wideHeight, got.modelsInput.Height())
	}
	if got.modelsInput.Width() >= wideWidth {
		t.Fatalf("expected narrower resize to reduce width below %d, got %d", wideWidth, got.modelsInput.Width())
	}
}

func TestModelsInputExactFitHeightMatchesRenderedRows(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	value := "1234567890"
	paneWidth := lipgloss.Width(value) + lipgloss.Width(modelsInputPrompt) + paneStyle.GetPaddingLeft() + paneStyle.GetPaddingRight()
	m.modelsInput.SetValue(value)
	syncModelsInputLayout(&m.modelsInput, paneWidth)
	m.modelsInput.Blur()

	gotHeight := m.modelsInput.Height()
	rendered := renderedLines(m.modelsInput.View())
	if gotHeight != len(rendered) {
		t.Fatalf("expected exact-fit height %d to match rendered rows %d: %#v", gotHeight, len(rendered), rendered)
	}
}

func TestModelsInputTypingRecomputesWrappedHeight(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 80
	setFocusedFormField(&m, addProviderModelsField)

	for _, r := range []rune(strings.Repeat("long-model-name-", 4)) {
		updated, cmd := m.Update(keyRunes(r))
		m = updated.(appModel)
		if cmd == nil {
			t.Fatal("expected textarea typing command")
		}
	}

	if m.modelsInput.Height() <= 1 {
		t.Fatalf("expected typing to grow wrapped height beyond 1, got %d", m.modelsInput.Height())
	}
	if want := modelsInputHeightForValue(m.modelsInput.Value(), listPaneWidth(m.width)); m.modelsInput.Height() != want {
		t.Fatalf("expected synced Models height %d, got %d", want, m.modelsInput.Height())
	}
}

func TestModelsInputGrowthKeepsFirstWrappedLineVisible(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 80
	setFocusedFormField(&m, addProviderModelsField)

	value := strings.Repeat("long-model-name-", 4)
	for _, r := range []rune(value) {
		updated, cmd := m.Update(keyRunes(r))
		m = updated.(appModel)
		if cmd == nil {
			t.Fatal("expected textarea typing command")
		}
	}

	if m.modelsInput.Height() <= 1 {
		t.Fatalf("expected wrapped typing to grow height beyond 1, got %d", m.modelsInput.Height())
	}

	wrapped := wrapTextareaLine([]rune(value), m.modelsInput.Width())
	if len(wrapped) <= 1 {
		t.Fatalf("test setup invalid: expected wrapped rows, got %d", len(wrapped))
	}

	lines := renderedLines(m.modelsInput.View())
	if got, want := lines[0], modelsInputPrompt+string(wrapped[0]); got != want {
		t.Fatalf("expected first visible row %q, got %q", want, got)
	}
	if !strings.Contains(lines[1], strings.TrimRight(string(wrapped[1]), " ")) {
		t.Fatalf("expected continuation row to keep wrapped content %q, got %q", strings.TrimRight(string(wrapped[1]), " "), lines[1])
	}
}

func TestModelsInputPasteGrowthKeepsFirstWrappedLineVisible(t *testing.T) {
	value := strings.Repeat("long-model-name-", 4)
	if err := clipboard.WriteAll(value); err != nil {
		t.Skipf("clipboard unavailable: %v", err)
	}

	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 80
	setFocusedFormField(&m, addProviderModelsField)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	m = updated.(appModel)
	if cmd == nil {
		t.Fatal("expected textarea paste command")
	}

	updated, cmd = m.Update(cmd())
	m = updated.(appModel)
	if cmd == nil {
		t.Fatal("expected paste message handling to preserve cursor command")
	}

	if got := m.modelsInput.Value(); got != value {
		t.Fatalf("expected pasted models value %q, got %q", value, got)
	}
	if m.modelsInput.Height() <= 1 {
		t.Fatalf("expected pasted wrapped content to grow height beyond 1, got %d", m.modelsInput.Height())
	}

	wrapped := wrapTextareaLine([]rune(value), m.modelsInput.Width())
	lines := renderedLines(m.modelsInput.View())
	if got, want := lines[0], modelsInputPrompt+string(wrapped[0]); got != want {
		t.Fatalf("expected first visible pasted row %q, got %q", want, got)
	}
	if !strings.Contains(lines[1], strings.TrimRight(string(wrapped[1]), " ")) {
		t.Fatalf("expected pasted continuation row to keep wrapped content %q, got %q", strings.TrimRight(string(wrapped[1]), " "), lines[1])
	}
}

func TestNewModelSeedsPromptInputWithDefault(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")

	if got := m.promptInput.Value(); got != juicyPrompt {
		t.Fatalf("expected default prompt %q, got %q", juicyPrompt, got)
	}
	if m.promptEditing {
		t.Fatal("expected prompt editing to start inactive")
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

func TestModelEnterUsesEditedPromptForChecks(t *testing.T) {
	var capturedPrompt string
	m := newModel(appConfig{Providers: []provider{{
		BaseURL: serverURL(t, func(w http.ResponseWriter, r *http.Request) {
			var req chatCompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			capturedPrompt = req.Messages[0].Content
			fmt.Fprint(w, `{"choices":[{"message":{"content":"1"}}]}`)
		}),
		Models: []string{"gpt-4o-mini"},
	}}}, "juicy-providers.json")
	m.promptInput.SetValue("edited juicy prompt")

	cmd := m.startChecks()
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if !m.running {
		t.Fatal("expected running to be true")
	}

	msg := cmd()
	if _, ok := msg.(runFinishedMsg); !ok {
		t.Fatalf("expected runFinishedMsg, got %T", msg)
	}
	if capturedPrompt != "edited juicy prompt" {
		t.Fatalf("expected edited prompt to be used, got %q", capturedPrompt)
	}
}

func TestModelTabFocusesPromptAndTypingUpdatesIt(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(appModel)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if !got.promptEditing {
		t.Fatal("expected prompt editing to be active")
	}

	updated, cmd = got.Update(keyRunes('!'))
	got = updated.(appModel)
	if cmd == nil {
		t.Fatal("expected textinput command while editing prompt")
	}
	if !strings.HasSuffix(got.promptInput.Value(), "!") {
		t.Fatalf("expected prompt to be updated, got %q", got.promptInput.Value())
	}

	updated, cmd = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(appModel)
	if cmd != nil {
		t.Fatal("expected no command when finishing prompt edit")
	}
	if got.promptEditing {
		t.Fatal("expected prompt editing to finish on enter")
	}
	if got.running {
		t.Fatal("expected finishing prompt edit not to start checks")
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
	m.baseURLInput.SetValue("https://example.com/v1")
	m.apiKeyInput.SetValue("secret")
	m.modelsInput.SetValue("gpt-4o-mini")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
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

func TestModelAddAndEditEntryStatusMentionsCtrlS(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		m := newModel(appConfig{}, "juicy-providers.json")

		updated, cmd := m.Update(keyRunes('a'))
		got := updated.(appModel)

		if cmd != nil {
			t.Fatal("expected no command")
		}
		if got.mode != addMode {
			t.Fatalf("expected add mode, got %v", got.mode)
		}
		if got.status != "新增供应商：模型支持逗号或换行；按 Ctrl+S 保存，Esc 取消。" {
			t.Fatalf("unexpected add status: %q", got.status)
		}
		if got.statusKind != statusInfo {
			t.Fatalf("expected info status kind, got %v", got.statusKind)
		}
	})

	t.Run("edit", func(t *testing.T) {
		m := newModel(appConfig{Providers: []provider{{
			BaseURL: "https://example.com/v1",
			APIKey:  "secret",
			Models:  []string{"gpt-4o-mini"},
		}}}, "juicy-providers.json")

		updated, cmd := m.Update(keyRunes('e'))
		got := updated.(appModel)

		if cmd != nil {
			t.Fatal("expected no command")
		}
		if got.mode != addMode {
			t.Fatalf("expected shared form mode, got %v", got.mode)
		}
		if got.status != "编辑供应商：修改基础 URL、API 密钥和模型；按 Ctrl+S 更新，Esc 取消。" {
			t.Fatalf("unexpected edit status: %q", got.status)
		}
		if got.statusKind != statusInfo {
			t.Fatalf("expected info status kind, got %v", got.statusKind)
		}
	})
}

func TestModelEditProviderWithNoProvidersShowsWarningAndStaysInListMode(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")

	updated, cmd := m.Update(keyRunes('e'))
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != listMode {
		t.Fatalf("expected list mode, got %v", got.mode)
	}
	if got.editingIndex != noEditingProviderIndex {
		t.Fatalf("expected no editing target, got %d", got.editingIndex)
	}
	if got.status != "还没有可编辑的供应商，请先新增。" {
		t.Fatalf("unexpected status: %q", got.status)
	}
	if got.statusKind != statusWarning {
		t.Fatalf("expected warning status kind, got %v", got.statusKind)
	}
}

func TestModelEditModePrefillsSelectedProviderAndReusesSharedPane(t *testing.T) {
	m := newModel(appConfig{Providers: []provider{{
		BaseURL: "https://one.example.com/v1",
		APIKey:  "first-secret",
		Models:  []string{"gpt-4o-mini"},
	}, {
		BaseURL: "https://two.example.com/v1",
		APIKey:  "second-secret",
		Models:  []string{"qwen-max", "claude-3.5-sonnet"},
	}}}, "juicy-providers.json")
	m.cursor = 1
	m.width = 100
	m.height = 24

	updated, cmd := m.Update(keyRunes('e'))
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != addMode {
		t.Fatalf("expected shared form mode, got %v", got.mode)
	}
	if got.editingIndex != 1 {
		t.Fatalf("expected editing index 1, got %d", got.editingIndex)
	}
	if got.baseURLInput.Value() != "https://two.example.com/v1" {
		t.Fatalf("unexpected prefilled Base URL: %q", got.baseURLInput.Value())
	}
	if got.apiKeyInput.Value() != "second-secret" {
		t.Fatalf("unexpected prefilled API key: %q", got.apiKeyInput.Value())
	}
	if got.modelsInput.Value() != "qwen-max\nclaude-3.5-sonnet" {
		t.Fatalf("unexpected prefilled models: %q", got.modelsInput.Value())
	}

	view := got.View()
	assertContainsAll(t, view,
		renderPaneTitle("供应商"),
		renderPaneTitle("编辑供应商"),
		selectionStyle.Render("https://two.example.com/v1（2 个模型）"),
		"qwen-max",
		"claude-3.5-sonnet",
		renderShortcutFooter(editModeFooterText()),
	)
	if strings.Contains(view, renderPaneTitle("结果")) {
		t.Fatalf("expected shared form pane to replace results pane: %q", view)
	}
	assertContainsAll(t, view,
		"修改当前供应商的 OAI 兼容 base URL、API",
		"key，以及支持逗号或换行的模型列表。",
	)
}

func TestModelEditSaveUpdatesProviderInPlace(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "providers.json")
	initial := appConfig{Providers: []provider{{
		BaseURL: "https://one.example.com/v1",
		APIKey:  "first-secret",
		Models:  []string{"gpt-4o-mini"},
	}, {
		BaseURL: "https://two.example.com/v1",
		APIKey:  "second-secret",
		Models:  []string{"qwen-max"},
	}}}
	if err := saveConfig(configPath, initial); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	m := newModel(initial, configPath)
	m.cursor = 1

	updated, _ := m.Update(keyRunes('e'))
	got := updated.(appModel)
	got.baseURLInput.SetValue("https://updated.example.com/v1/")
	got.apiKeyInput.SetValue("updated-secret")
	got.modelsInput.SetValue("qwen-max\nclaude-3.5-sonnet")
	setFocusedFormField(&got, addProviderAPIKeyField)

	updated, cmd := got.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	saved := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if saved.mode != listMode {
		t.Fatalf("expected list mode, got %v", saved.mode)
	}
	if len(saved.config.Providers) != 2 {
		t.Fatalf("expected provider count to remain 2, got %d", len(saved.config.Providers))
	}
	if saved.config.Providers[0].BaseURL != initial.Providers[0].BaseURL || saved.config.Providers[0].APIKey != initial.Providers[0].APIKey || len(saved.config.Providers[0].Models) != len(initial.Providers[0].Models) || saved.config.Providers[0].Models[0] != initial.Providers[0].Models[0] {
		t.Fatalf("expected first provider unchanged, got %+v", saved.config.Providers[0])
	}
	if saved.config.Providers[1].BaseURL != "https://updated.example.com/v1" {
		t.Fatalf("unexpected updated Base URL: %q", saved.config.Providers[1].BaseURL)
	}
	if saved.config.Providers[1].APIKey != "updated-secret" {
		t.Fatalf("unexpected updated API key: %q", saved.config.Providers[1].APIKey)
	}
	if gotModels, wantModels := saved.config.Providers[1].Models, []string{"qwen-max", "claude-3.5-sonnet"}; len(gotModels) != len(wantModels) || gotModels[0] != wantModels[0] || gotModels[1] != wantModels[1] {
		t.Fatalf("unexpected updated models: got %#v want %#v", gotModels, wantModels)
	}
	if saved.status != "已更新供应商 https://updated.example.com/v1，共 2 个模型。" {
		t.Fatalf("unexpected status: %q", saved.status)
	}
	if saved.statusKind != statusSuccess {
		t.Fatalf("expected success status kind, got %v", saved.statusKind)
	}

	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if len(loaded.Providers) != 2 {
		t.Fatalf("expected persisted provider count 2, got %d", len(loaded.Providers))
	}
	if loaded.Providers[1].BaseURL != "https://updated.example.com/v1" {
		t.Fatalf("unexpected persisted Base URL: %q", loaded.Providers[1].BaseURL)
	}
}

func TestModelEnterOnBaseURLOrAPIKeyDoesNotUpdateProviderInEditMode(t *testing.T) {
	for _, tc := range []struct {
		name       string
		focusIndex int
	}{
		{name: "base-url", focusIndex: addProviderBaseURLField},
		{name: "api-key", focusIndex: addProviderAPIKeyField},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "providers.json")
			initial := appConfig{Providers: []provider{{
				BaseURL: "https://one.example.com/v1",
				APIKey:  "first-secret",
				Models:  []string{"gpt-4o-mini"},
			}, {
				BaseURL: "https://two.example.com/v1",
				APIKey:  "second-secret",
				Models:  []string{"qwen-max"},
			}}}
			if err := saveConfig(configPath, initial); err != nil {
				t.Fatalf("seed config: %v", err)
			}

			m := newModel(initial, configPath)
			m.cursor = 1

			updated, _ := m.Update(keyRunes('e'))
			got := updated.(appModel)
			got.baseURLInput.SetValue("https://updated.example.com/v1")
			got.apiKeyInput.SetValue("updated-secret")
			got.modelsInput.SetValue("qwen-max\nclaude-3.5-sonnet")
			setFocusedFormField(&got, tc.focusIndex)

			updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
			unchanged := updated.(appModel)

			if unchanged.mode != addMode {
				t.Fatalf("expected to remain in shared form mode, got %v", unchanged.mode)
			}
			if unchanged.editingIndex != 1 {
				t.Fatalf("expected editing index preserved, got %d", unchanged.editingIndex)
			}
			if unchanged.config.Providers[1].BaseURL != initial.Providers[1].BaseURL {
				t.Fatalf("expected provider Base URL unchanged, got %q", unchanged.config.Providers[1].BaseURL)
			}
			if unchanged.config.Providers[1].APIKey != initial.Providers[1].APIKey {
				t.Fatalf("expected provider API key unchanged, got %q", unchanged.config.Providers[1].APIKey)
			}
			if len(unchanged.config.Providers[1].Models) != len(initial.Providers[1].Models) || unchanged.config.Providers[1].Models[0] != initial.Providers[1].Models[0] {
				t.Fatalf("expected provider models unchanged, got %#v", unchanged.config.Providers[1].Models)
			}
			if unchanged.baseURLInput.Value() != "https://updated.example.com/v1" {
				t.Fatalf("expected edited Base URL to remain in form, got %q", unchanged.baseURLInput.Value())
			}
			if unchanged.apiKeyInput.Value() != "updated-secret" {
				t.Fatalf("expected edited API key to remain in form, got %q", unchanged.apiKeyInput.Value())
			}
			if unchanged.modelsInput.Value() != "qwen-max\nclaude-3.5-sonnet" {
				t.Fatalf("expected edited models to remain in form, got %q", unchanged.modelsInput.Value())
			}

			loaded, err := loadConfig(configPath)
			if err != nil {
				t.Fatalf("load saved config: %v", err)
			}
			if loaded.Providers[1].BaseURL != initial.Providers[1].BaseURL {
				t.Fatalf("expected persisted Base URL unchanged, got %q", loaded.Providers[1].BaseURL)
			}
			if loaded.Providers[1].APIKey != initial.Providers[1].APIKey {
				t.Fatalf("expected persisted API key unchanged, got %q", loaded.Providers[1].APIKey)
			}
		})
	}
}

func TestModelEditSavePreservesCursorSelection(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "providers.json")
	initial := appConfig{Providers: []provider{{
		BaseURL: "https://one.example.com/v1",
		APIKey:  "first-secret",
		Models:  []string{"gpt-4o-mini"},
	}, {
		BaseURL: "https://two.example.com/v1",
		APIKey:  "second-secret",
		Models:  []string{"qwen-max"},
	}}}
	if err := saveConfig(configPath, initial); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	m := newModel(initial, configPath)
	m.width = 100
	m.cursor = 1

	updated, _ := m.Update(keyRunes('e'))
	got := updated.(appModel)
	got.modelsInput.SetValue("qwen-max\nclaude-3.5-sonnet")
	setFocusedFormField(&got, addProviderAPIKeyField)

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	saved := updated.(appModel)

	if saved.cursor != 1 {
		t.Fatalf("expected cursor to remain on edited provider, got %d", saved.cursor)
	}
	view := saved.View()
	if !strings.Contains(view, selectionStyle.Render("https://two.example.com/v1（2 个模型）")) {
		t.Fatalf("expected edited provider to remain selected in list view: %q", view)
	}
}

func TestModelEditSaveFailureRollsBackAndStaysInFormMode(t *testing.T) {
	original := provider{
		BaseURL: "https://one.example.com/v1",
		APIKey:  "first-secret",
		Models:  []string{"gpt-4o-mini"},
	}
	m := newModel(appConfig{Providers: []provider{original}}, t.TempDir())
	m.cursor = 0

	updated, _ := m.Update(keyRunes('e'))
	got := updated.(appModel)
	got.baseURLInput.SetValue("https://updated.example.com/v1")
	got.apiKeyInput.SetValue("updated-secret")
	got.modelsInput.SetValue("qwen-max")
	setFocusedFormField(&got, addProviderAPIKeyField)

	updated, cmd := got.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	failed := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if failed.mode != addMode {
		t.Fatalf("expected to remain in shared form mode, got %v", failed.mode)
	}
	if failed.editingIndex != 0 {
		t.Fatalf("expected editing index preserved, got %d", failed.editingIndex)
	}
	if len(failed.config.Providers) != 1 {
		t.Fatalf("expected provider count unchanged, got %d", len(failed.config.Providers))
	}
	if failed.config.Providers[0].BaseURL != original.BaseURL || failed.config.Providers[0].APIKey != original.APIKey || len(failed.config.Providers[0].Models) != len(original.Models) || failed.config.Providers[0].Models[0] != original.Models[0] {
		t.Fatalf("expected provider rollback, got %+v", failed.config.Providers[0])
	}
	if failed.baseURLInput.Value() != "https://updated.example.com/v1" {
		t.Fatalf("expected edited Base URL to remain in form, got %q", failed.baseURLInput.Value())
	}
	if !strings.HasPrefix(failed.status, "更新供应商失败：") {
		t.Fatalf("unexpected status: %q", failed.status)
	}
	if failed.statusKind != statusError {
		t.Fatalf("expected error status kind, got %v", failed.statusKind)
	}
}

func TestModelEditModeEscCancelsAndPreservesSelection(t *testing.T) {
	m := newModel(appConfig{Providers: []provider{{
		BaseURL: "https://one.example.com/v1",
		APIKey:  "first-secret",
		Models:  []string{"gpt-4o-mini"},
	}, {
		BaseURL: "https://two.example.com/v1",
		APIKey:  "second-secret",
		Models:  []string{"qwen-max"},
	}}}, "juicy-providers.json")
	m.width = 100
	m.cursor = 1

	updated, _ := m.Update(keyRunes('e'))
	got := updated.(appModel)
	got.baseURLInput.SetValue("https://unsaved.example.com/v1")

	updated, cmd := got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	canceled := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if canceled.mode != listMode {
		t.Fatalf("expected list mode, got %v", canceled.mode)
	}
	if canceled.cursor != 1 {
		t.Fatalf("expected cursor to stay on edited provider, got %d", canceled.cursor)
	}
	if canceled.editingIndex != noEditingProviderIndex {
		t.Fatalf("expected editing context cleared, got %d", canceled.editingIndex)
	}
	if canceled.status != "已取消编辑供应商。" {
		t.Fatalf("unexpected status: %q", canceled.status)
	}
	if canceled.statusKind != statusInfo {
		t.Fatalf("expected info status kind, got %v", canceled.statusKind)
	}
	view := canceled.View()
	if !strings.Contains(view, renderPaneTitle("结果")) {
		t.Fatalf("expected results pane restored after cancel: %q", view)
	}
	if strings.Contains(view, renderPaneTitle("编辑供应商")) {
		t.Fatalf("expected edit pane removed after cancel: %q", view)
	}
	if !strings.Contains(view, selectionStyle.Render("https://two.example.com/v1（1 个模型）")) {
		t.Fatalf("expected edited provider selection preserved in view: %q", view)
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

func TestModelRunningBlocksEditEntry(t *testing.T) {
	m := newModel(appConfig{Providers: []provider{{BaseURL: "https://one", Models: []string{"a"}}}}, "juicy-providers.json")
	m.running = true

	updated, cmd := m.Update(keyRunes('e'))
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != listMode {
		t.Fatalf("expected list mode, got %v", got.mode)
	}
	if got.editingIndex != noEditingProviderIndex {
		t.Fatalf("expected no editing target, got %d", got.editingIndex)
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

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
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
	if got.baseURLInput.Placeholder == "" {
		t.Fatal("expected English placeholder to be visible")
	}
	if got.modelsInput.Placeholder == "" {
		t.Fatal("expected English models placeholder to be visible")
	}
}

func TestModelAddModeEscCancelsAndReturnsToList(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 100

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

	view := got.View()
	if !strings.Contains(view, renderPaneTitle("结果")) {
		t.Fatalf("expected results pane after cancel: %q", view)
	}
	if strings.Contains(view, renderPaneTitle("新增供应商")) {
		t.Fatalf("expected add-provider pane removed after cancel: %q", view)
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

func TestModelEnterInModelsTextareaAddsNewlineWithoutSaving(t *testing.T) {
	m := newModel(appConfig{}, filepath.Join(t.TempDir(), "providers.json"))
	m.mode = addMode
	m.baseURLInput.SetValue("https://example.com/v1")
	m.apiKeyInput.SetValue("secret")
	m.modelsInput.SetValue("gpt-4o-mini")
	setFocusedFormField(&m, addProviderModelsField)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(appModel)

	if got.mode != addMode {
		t.Fatalf("expected to remain in add mode, got %v", got.mode)
	}
	if got.focusIndex != addProviderModelsField {
		t.Fatalf("expected models field to stay focused, got %d", got.focusIndex)
	}
	if len(got.config.Providers) != 0 {
		t.Fatalf("expected no provider save, got %d providers", len(got.config.Providers))
	}
	if got.modelsInput.Value() != "gpt-4o-mini\n" {
		t.Fatalf("expected textarea enter to add a newline, got %q", got.modelsInput.Value())
	}
	if got.modelsInput.Height() != 2 {
		t.Fatalf("expected textarea enter to resync height to 2 rows, got %d", got.modelsInput.Height())
	}
}

func TestModelEnterOnBaseURLOrAPIKeyDoesNotSaveProviderInAddMode(t *testing.T) {
	for _, tc := range []struct {
		name       string
		focusIndex int
	}{
		{name: "base-url", focusIndex: addProviderBaseURLField},
		{name: "api-key", focusIndex: addProviderAPIKeyField},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "providers.json")
			m := newModel(appConfig{}, configPath)
			m.mode = addMode
			m.baseURLInput.SetValue("https://example.com/v1")
			m.apiKeyInput.SetValue("secret")
			m.modelsInput.SetValue("gpt-4o-mini")
			setFocusedFormField(&m, tc.focusIndex)

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			got := updated.(appModel)

			if got.mode != addMode {
				t.Fatalf("expected to remain in add mode, got %v", got.mode)
			}
			if got.editingIndex != noEditingProviderIndex {
				t.Fatalf("expected add context to stay active, got editing index %d", got.editingIndex)
			}
			if len(got.config.Providers) != 0 {
				t.Fatalf("expected no provider save, got %d providers", len(got.config.Providers))
			}
			if got.baseURLInput.Value() != "https://example.com/v1" {
				t.Fatalf("unexpected Base URL value after enter: %q", got.baseURLInput.Value())
			}
			if got.apiKeyInput.Value() != "secret" {
				t.Fatalf("unexpected API key value after enter: %q", got.apiKeyInput.Value())
			}
			if got.modelsInput.Value() != "gpt-4o-mini" {
				t.Fatalf("unexpected models value after enter: %q", got.modelsInput.Value())
			}
		})
	}
}

func TestModelTypingLowercaseLInAddModeDoesNotToggleLanguage(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.lang = langEN
	setFocusedFormField(&m, addProviderModelsField)

	updated, cmd := m.Update(keyRunes('l'))
	got := updated.(appModel)

	if cmd == nil {
		t.Fatal("expected textarea typing command")
	}
	if got.lang != langEN {
		t.Fatalf("expected language unchanged, got %v", got.lang)
	}
	if got.modelsInput.Value() != "l" {
		t.Fatalf("expected lowercase l to be typed into models textarea, got %q", got.modelsInput.Value())
	}
}

func TestModelTextareaSupportsTypingAndArrowNavigationWithinField(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	setFocusedFormField(&m, addProviderModelsField)

	updated, cmd := m.Update(keyRunes('a'))
	got := updated.(appModel)
	if cmd == nil {
		t.Fatal("expected textarea typing command")
	}
	if got.modelsInput.Value() != "a" {
		t.Fatalf("expected textarea value to update, got %q", got.modelsInput.Value())
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(appModel)
	if got.modelsInput.Value() != "a\n" {
		t.Fatalf("expected newline in textarea, got %q", got.modelsInput.Value())
	}

	updated, cmd = got.Update(keyRunes('b'))
	got = updated.(appModel)
	if cmd == nil {
		t.Fatal("expected textarea typing command after newline")
	}
	if got.modelsInput.Value() != "a\nb" {
		t.Fatalf("expected second line text in textarea, got %q", got.modelsInput.Value())
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyUp})
	got = updated.(appModel)
	if got.focusIndex != addProviderModelsField {
		t.Fatalf("expected up arrow to stay in models textarea, got focus %d", got.focusIndex)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = updated.(appModel)
	if got.focusIndex != addProviderModelsField {
		t.Fatalf("expected down arrow to stay in models textarea, got focus %d", got.focusIndex)
	}
}

func TestModelEscCancelsFromEveryFocusedField(t *testing.T) {
	for _, focusIndex := range []int{addProviderBaseURLField, addProviderAPIKeyField, addProviderModelsField} {
		t.Run(fmt.Sprintf("focus-%d", focusIndex), func(t *testing.T) {
			m := newModel(appConfig{}, "juicy-providers.json")
			m.mode = addMode
			setFocusedFormField(&m, focusIndex)

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
		})
	}
}

func TestModelCtrlSSavesProviderWithMultilineModels(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "providers.json")
	m := newModel(appConfig{}, configPath)
	m.mode = addMode
	m.baseURLInput.SetValue("https://example.com/v1/")
	m.apiKeyInput.SetValue("secret")
	m.modelsInput.SetValue("gpt-4o-mini\nqwen-max, claude-3.5-sonnet\nqwen-max")
	setFocusedFormField(&m, addProviderAPIKeyField)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != listMode {
		t.Fatalf("expected list mode, got %v", got.mode)
	}
	if len(got.config.Providers) != 1 {
		t.Fatalf("expected one provider, got %d", len(got.config.Providers))
	}
	wantModels := []string{"gpt-4o-mini", "qwen-max", "claude-3.5-sonnet"}
	if len(got.config.Providers[0].Models) != len(wantModels) {
		t.Fatalf("unexpected saved model count: got %d want %d", len(got.config.Providers[0].Models), len(wantModels))
	}
	for i := range wantModels {
		if got.config.Providers[0].Models[i] != wantModels[i] {
			t.Fatalf("unexpected saved model at %d: got %q want %q", i, got.config.Providers[0].Models[i], wantModels[i])
		}
	}
	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if len(loaded.Providers) != 1 {
		t.Fatalf("expected one saved provider, got %d", len(loaded.Providers))
	}
	for i := range wantModels {
		if loaded.Providers[0].Models[i] != wantModels[i] {
			t.Fatalf("unexpected persisted model at %d: got %q want %q", i, loaded.Providers[0].Models[i], wantModels[i])
		}
	}
}

func TestModelRejectsInvalidURLAndEmptyModels(t *testing.T) {
	t.Run("invalid URL", func(t *testing.T) {
		m := newModel(appConfig{}, "juicy-providers.json")
		m.mode = addMode
		m.baseURLInput.SetValue("example.com")
		m.modelsInput.SetValue("gpt-4o-mini")

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
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
		m.baseURLInput.SetValue("https://example.com/v1")

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
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
	m.width = 100
	m.baseURLInput.SetValue("https://example.com/v1/")
	m.apiKeyInput.SetValue("secret")
	m.modelsInput.SetValue("gpt-4o-mini, qwen-max")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
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
	view := got.View()
	if !strings.Contains(view, renderPaneTitle("结果")) {
		t.Fatalf("expected results pane restored after save: %q", view)
	}
	if strings.Contains(view, renderPaneTitle("新增供应商")) {
		t.Fatalf("expected add-provider pane removed after save: %q", view)
	}
	if !strings.Contains(view, selectionStyle.Render("https://example.com/v1（2 个模型）")) {
		t.Fatalf("expected saved provider to remain visible in provider pane: %q", view)
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
			if !strings.Contains(view, fieldLabelStyle.Render("提示词：")) {
				t.Fatalf("expected prompt label in view: %q", view)
			}
			if !strings.Contains(view, juicyPrompt) {
				t.Fatalf("expected default prompt in view: %q", view)
			}
			if !strings.Contains(view, renderPaneTitle("供应商")) {
				t.Fatalf("expected provider pane title in view: %q", view)
			}
			if !strings.Contains(view, renderPaneTitle("结果")) {
				t.Fatalf("expected result pane title in view: %q", view)
			}
			if !strings.Contains(view, renderEmptyState("还没有保存任何供应商，按 'a' 新增。")) {
				t.Fatalf("expected provider empty state in view: %q", view)
			}
			if !strings.Contains(view, "暂无结果，请先选择供应商并按") || !strings.Contains(view, "Enter。") {
				t.Fatalf("expected result empty state in view: %q", view)
			}
			if !strings.Contains(view, renderShortcutFooter(listModeFooterText())) {
				t.Fatalf("expected shortcut footer in view: %q", view)
			}
		})
	}
}

func TestRenderTitledPaneWithHeightClampsWrappedSingleLineContent(t *testing.T) {
	width := 24
	height := 8
	body := strings.Repeat("https://provider.example.com/v1/models/super-long-name", 2)

	if strings.Contains(body, "\n") {
		t.Fatalf("expected single-line body, got %q", body)
	}

	full := renderTitledPane("Providers", width, body)
	limited := renderTitledPaneWithHeight("Providers", width, height, body)
	lines := strings.Split(limited, "\n")
	border := lipgloss.RoundedBorder()

	if lipgloss.Height(full) <= height {
		t.Fatalf("test setup invalid: wrapped pane height %d must exceed limit %d", lipgloss.Height(full), height)
	}
	if got := lipgloss.Height(limited); got != height {
		t.Fatalf("expected limited pane height %d, got %d", height, got)
	}
	if !strings.Contains(lines[0], renderPaneTitle("Providers")) {
		t.Fatalf("expected titled border preserved, got %q", lines[0])
	}
	if !strings.Contains(limited, "https://provider") {
		t.Fatalf("expected wrapped content preserved, got %q", limited)
	}
	if !strings.Contains(lines[len(lines)-1], border.BottomLeft) || !strings.Contains(lines[len(lines)-1], border.BottomRight) {
		t.Fatalf("expected bottom border on final line, got %q", lines[len(lines)-1])
	}
}

func TestRenderTitledPaneWithHeightAndRightScrollbarUsesBorderTrackOnOverflow(t *testing.T) {
	width := 24
	height := 8
	body := strings.Repeat("https://provider.example.com/v1/models/super-long-name", 2)
	rendered := renderTitledPaneWithHeightAndRightScrollbar("Add Provider", width, height, body, addProviderPaneBorderColor)
	lines := renderedLines(rendered)
	border := lipgloss.RoundedBorder()

	if !strings.Contains(lines[0], stripANSI(renderPaneTitle("Add Provider", addProviderPaneBorderColor))) {
		t.Fatalf("expected titled top border preserved, got %q", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], border.BottomLeft) || !strings.Contains(lines[len(lines)-1], border.BottomRight) {
		t.Fatalf("expected bottom border preserved, got %q", lines[len(lines)-1])
	}

	thumbRows := 0
	trackRows := 0
	for _, line := range lines[1 : len(lines)-1] {
		if first := firstRuneString(line); first != border.Left {
			t.Fatalf("expected left border preserved, got %q", line)
		}
		switch lastRuneString(line) {
		case "▌":
			thumbRows++
		case border.Right:
			trackRows++
		default:
			t.Fatalf("expected right border track glyph on body row, got %q", line)
		}
	}

	if thumbRows == 0 {
		t.Fatalf("expected scrollbar thumb rows on overflow, got %q", rendered)
	}
	if trackRows == 0 {
		t.Fatalf("expected scrollbar track rows outside the thumb, got %q", rendered)
	}
}

func TestRenderTitledPaneWithHeightAndRightScrollbarKeepsPlainBorderWithoutOverflow(t *testing.T) {
	width := 28
	height := 10
	body := "Base URL\nAPI key\nModels"
	rendered := renderTitledPaneWithHeightAndRightScrollbar("Add Provider", width, height, body, addProviderPaneBorderColor)
	lines := renderedLines(rendered)
	border := lipgloss.RoundedBorder()

	if strings.Contains(stripANSI(rendered), "▌") {
		t.Fatalf("expected no scrollbar thumb without overflow, got %q", rendered)
	}

	for _, line := range lines[1 : len(lines)-1] {
		if last := lastRuneString(line); last != border.Right {
			t.Fatalf("expected plain right border without overflow, got %q", line)
		}
	}
}

func TestPaneScrollbarThumbRangeReflectsVisibleRatioAndOffset(t *testing.T) {
	tests := []struct {
		name        string
		meta        paneScrollbarMeta
		trackHeight int
		wantStart   int
		wantEnd     int
	}{
		{
			name:        "middle-window",
			meta:        paneScrollbarMeta{totalLines: 20, visibleLines: 6, viewportOffset: 7},
			trackHeight: 10,
			wantStart:   4,
			wantEnd:     7,
		},
		{
			name:        "bottom-window",
			meta:        paneScrollbarMeta{totalLines: 20, visibleLines: 6, viewportOffset: 14},
			trackHeight: 10,
			wantStart:   7,
			wantEnd:     10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotStart, gotEnd, ok := tc.meta.thumbRange(tc.trackHeight)
			if !ok {
				t.Fatal("expected scrollbar thumb for overflowing content")
			}
			if gotStart != tc.wantStart || gotEnd != tc.wantEnd {
				t.Fatalf("expected thumb range [%d,%d), got [%d,%d)", tc.wantStart, tc.wantEnd, gotStart, gotEnd)
			}
		})
	}
}

func TestRenderTitledPaneWithHeightAndRightScrollbarPreservesContentWidth(t *testing.T) {
	width := 24
	height := 8
	body := strings.Repeat("https://provider.example.com/v1/models/super-long-name", 2)
	plainLines := renderedLines(renderTitledPaneWithHeight("Add Provider", width, height, body, addProviderPaneBorderColor))
	scrollbarLines := renderedLines(renderTitledPaneWithHeightAndRightScrollbar("Add Provider", width, height, body, addProviderPaneBorderColor))

	if len(plainLines) != len(scrollbarLines) {
		t.Fatalf("expected same line count, got plain=%d scrollbar=%d", len(plainLines), len(scrollbarLines))
	}

	for i := 1; i < len(plainLines)-1; i++ {
		if got, want := lipgloss.Width(scrollbarLines[i]), lipgloss.Width(plainLines[i]); got != want {
			t.Fatalf("expected line %d width %d, got %d", i, want, got)
		}
		if got, want := trimLastRune(scrollbarLines[i]), trimLastRune(plainLines[i]); got != want {
			t.Fatalf("expected line %d content preserved before right border\nwant: %q\ngot:  %q", i, want, got)
		}
	}
}

func TestRenderTitledPaneWithHeightAndRightScrollbarViewportMovesThumbWithOffset(t *testing.T) {
	width := 24
	height := 8
	body := strings.Join([]string{
		"line-01", "line-02", "line-03", "line-04", "line-05", "line-06",
		"line-07", "line-08", "line-09", "line-10", "line-11", "line-12",
	}, "\n")
	top := thumbRows(renderedLines(renderTitledPaneWithHeightAndRightScrollbarViewport("Add Provider", width, height, body, 0, addProviderPaneBorderColor)))
	scrolled := thumbRows(renderedLines(renderTitledPaneWithHeightAndRightScrollbarViewport("Add Provider", width, height, body, 4, addProviderPaneBorderColor)))

	if len(top) == 0 || len(scrolled) == 0 {
		t.Fatal("expected scrollbar thumb rows for overflowing pane")
	}
	if scrolled[0] <= top[0] {
		t.Fatalf("expected scrolled viewport thumb to move downward, got top=%v scrolled=%v", top, scrolled)
	}
}

func TestRightBorderScrollbarStaysScopedToAddEditPane(t *testing.T) {
	width := 24
	height := 8
	body := strings.Repeat("https://provider.example.com/v1/models/super-long-name", 2)

	providerPane := renderTitledPaneWithHeight("Providers", width, height, body)
	resultsPane := renderTitledPaneWithHeight("Results", width, height, body, resultsPaneBorderColor)
	formPane := renderTitledPaneWithHeightAndRightScrollbar("Add Provider", width, height, body, addProviderPaneBorderColor)

	if strings.Contains(stripANSI(providerPane), "▌") {
		t.Fatalf("expected providers pane to keep its plain border, got %q", providerPane)
	}
	if strings.Contains(stripANSI(resultsPane), "▌") {
		t.Fatalf("expected results pane to keep its plain border, got %q", resultsPane)
	}
	if !strings.Contains(stripANSI(formPane), "▌") {
		t.Fatalf("expected add/edit pane to render a scrollbar on overflow, got %q", formPane)
	}
}

func TestFormViewRoutesScrollbarToAddPaneWhileListViewStaysPlain(t *testing.T) {
	m := newModel(appConfig{Providers: []provider{
		{BaseURL: "https://one", Models: []string{"gpt-4o-mini", "qwen-max", "claude-3.5-sonnet"}},
		{BaseURL: "https://two", Models: []string{"gpt-4o-mini", "qwen-max", "claude-3.5-sonnet"}},
		{BaseURL: "https://three", Models: []string{"gpt-4o-mini", "qwen-max", "claude-3.5-sonnet"}},
	}}, "juicy-providers.json")
	m.width = 100
	m.height = 16
	m.modelsInput.SetValue("gpt-4o-mini\nqwen-max\nclaude-3.5-sonnet\nllama-3.1-8b\nqvq-plus\ndeepseek-chat")
	setFocusedFormField(&m, addProviderModelsField)

	listView := stripANSI(m.listView())
	if strings.Contains(listView, "▌") {
		t.Fatalf("expected list view panes to keep plain borders, got %q", listView)
	}

	m.mode = addMode
	formView := stripANSI(m.formView())
	if !strings.Contains(formView, "▌") {
		t.Fatalf("expected add/edit form view to render the right-border scrollbar, got %q", formView)
	}
}

func TestEnterAddAndEditModeResetFormPaneScrollOffset(t *testing.T) {
	m := newModel(appConfig{Providers: []provider{{BaseURL: "https://one", APIKey: "secret", Models: []string{"gpt-4o-mini", "qwen-max"}}}}, "juicy-providers.json")
	m.width = 100
	m.height = 16
	m.formPaneScrollOffset = 5

	updated, cmd := m.Update(keyRunes('a'))
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != addMode {
		t.Fatalf("expected add mode, got %v", got.mode)
	}
	if got.formPaneScrollOffset != 0 {
		t.Fatalf("expected add mode to reset form pane scroll, got %d", got.formPaneScrollOffset)
	}

	got.mode = listMode
	got.formPaneScrollOffset = 7
	updated, cmd = got.Update(keyRunes('e'))
	got = updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != addMode {
		t.Fatalf("expected edit mode to enter addMode view, got %v", got.mode)
	}
	if got.formPaneScrollOffset != 0 {
		t.Fatalf("expected edit mode to reset form pane scroll, got %d", got.formPaneScrollOffset)
	}
}

func TestModelsTypingScrollsFormPaneToKeepCursorVisible(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 100
	m.height = 14
	setFocusedFormField(&m, addProviderModelsField)

	for i := 1; i <= 8; i++ {
		for _, r := range []rune(fmt.Sprintf("model-%02d", i)) {
			updated, _ := m.Update(keyRunes(r))
			m = updated.(appModel)
		}
		if i < 8 {
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(appModel)
		}
	}

	if m.formPaneScrollOffset <= 0 {
		t.Fatalf("expected models typing to scroll the form pane, got %d", m.formPaneScrollOffset)
	}
	assertActiveFormCursorVisible(t, m)
	if !strings.Contains(stripANSI(m.formView()), "model-08") {
		t.Fatalf("expected rendered form view to keep the active models content visible, got %q", m.formView())
	}
}

func TestFocusChangeRepositionsFormPaneScrollForNewActiveField(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 100
	m.height = 14
	m.modelsInput.SetValue(strings.Join([]string{
		"model-01", "model-02", "model-03", "model-04", "model-05", "model-06", "model-07", "model-08",
	}, "\n"))
	setFocusedFormField(&m, addProviderModelsField)
	m.syncFormPaneScroll()
	if m.formPaneScrollOffset <= 0 {
		t.Fatalf("expected models focus to start from a scrolled pane, got %d", m.formPaneScrollOffset)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.focusIndex != addProviderAPIKeyField {
		t.Fatalf("expected focus to move to API key field, got %d", got.focusIndex)
	}
	if got.formPaneScrollOffset >= m.formPaneScrollOffset {
		t.Fatalf("expected focus change to scroll upward for the new field, before=%d after=%d", m.formPaneScrollOffset, got.formPaneScrollOffset)
	}
	assertActiveFormCursorVisible(t, got)
}

func TestListViewKeepsResultsPaneAboveBottomBarAcrossViewportHeights(t *testing.T) {
	border := lipgloss.RoundedBorder()

	tests := []struct {
		name         string
		heightOffset int
	}{
		{name: "tall-height", heightOffset: 4},
		{name: "exact-fit", heightOffset: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(appConfig{}, "juicy-providers.json")
			m.width = 100

			headerHeight, naturalBodyHeight, bottomHeight := listLayoutHeightsForTest(m)
			m.height = headerHeight + listHeaderGapHeight + bottomHeight + naturalBodyHeight + tc.heightOffset

			view := m.listView()
			lines := strings.Split(view, "\n")

			if got := lipgloss.Height(view); got != m.height {
				t.Fatalf("expected view height %d, got %d", m.height, got)
			}
			if strings.TrimRight(lines[len(lines)-1], " ") != renderShortcutFooter(listModeFooterText()) {
				t.Fatalf("expected footer on last line, got %q", lines[len(lines)-1])
			}
			if strings.TrimRight(lines[len(lines)-2], " ") != m.statusLine() {
				t.Fatalf("expected status on second-to-last line, got %q", lines[len(lines)-2])
			}

			bodyBottomLine := strings.TrimRight(lines[len(lines)-bottomHeight-1], " ")
			if !strings.Contains(bodyBottomLine, border.BottomLeft) || !strings.Contains(bodyBottomLine, border.BottomRight) {
				t.Fatalf("expected results pane bottom border directly above bottom bar, got %q", bodyBottomLine)
			}
		})
	}
}

func TestListViewKeepsFooterPinnedWhenProviderTextWraps(t *testing.T) {
	providerConfig := provider{
		BaseURL: "https://very-long-provider-hostname.example.com/openai-compatible/v1/chat/completions/with/a/path/that/keeps/wrapping/when/the/pane/is-narrow",
		Models:  []string{"gpt-4o-mini-super-long-model-name-that-wraps-again-and-again", "claude-compatible-model-name-that-also-wraps-a-lot"},
	}
	m := newModel(appConfig{Providers: []provider{providerConfig}}, "juicy-providers.json")
	m.width = 100

	headerHeight, _, bottomHeight := listLayoutHeightsForTest(m)
	bodyHeight := 10
	paneWidth := listPaneWidth(m.width)
	naturalProviderHeight := lipgloss.Height(renderTitledPane(m.tr("供应商", "Providers"), paneWidth, m.providerListView()))
	if naturalProviderHeight <= bodyHeight {
		t.Fatalf("test setup invalid: provider pane height %d must exceed body height %d", naturalProviderHeight, bodyHeight)
	}
	m.height = headerHeight + listHeaderGapHeight + bottomHeight + bodyHeight

	view := m.listView()
	lines := strings.Split(view, "\n")
	border := lipgloss.RoundedBorder()

	if got := lipgloss.Height(view); got != m.height {
		t.Fatalf("expected view height %d, got %d", m.height, got)
	}
	if strings.TrimRight(lines[len(lines)-1], " ") != renderShortcutFooter(listModeFooterText()) {
		t.Fatalf("expected footer on last line, got %q", lines[len(lines)-1])
	}
	if strings.TrimRight(lines[len(lines)-2], " ") != m.statusLine() {
		t.Fatalf("expected status on second-to-last line, got %q", lines[len(lines)-2])
	}

	bodyBottomLine := strings.TrimRight(lines[len(lines)-bottomHeight-1], " ")
	if !strings.Contains(bodyBottomLine, border.BottomLeft) || !strings.Contains(bodyBottomLine, border.BottomRight) {
		t.Fatalf("expected pane bottom border directly above bottom bar, got %q", bodyBottomLine)
	}
	if !strings.Contains(view, "very-long-provider-") {
		t.Fatalf("expected wrapped provider text in list view, got %q", view)
	}
}

func TestListViewClampsGracefullyForZeroAndSmallHeights(t *testing.T) {
	tests := []struct {
		name   string
		height int
	}{
		{name: "zero-height", height: 0},
		{name: "small-height", height: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(appConfig{}, "juicy-providers.json")
			m.width = 100
			m.height = tc.height

			assertNoPanic(t, func() {
				view := m.listView()
				lines := strings.Split(view, "\n")

				if strings.TrimRight(lines[len(lines)-1], " ") != renderShortcutFooter(listModeFooterText()) {
					t.Fatalf("expected footer on last line, got %q", lines[len(lines)-1])
				}
				if strings.TrimRight(lines[len(lines)-2], " ") != m.statusLine() {
					t.Fatalf("expected status on second-to-last line, got %q", lines[len(lines)-2])
				}

				headerHeight, _, bottomHeight := listLayoutHeightsForTest(m)
				availableHeight := m.availableListBodyHeight(
					m.renderPageHeaderWithPrompt(),
					m.listBottomContent(),
				)
				if availableHeight != 0 {
					t.Fatalf("expected clamped available height 0, got %d", availableHeight)
				}
				if headerHeight+listHeaderGapHeight+bottomHeight <= tc.height {
					t.Fatalf("test setup invalid: header and bottom bar should exceed viewport height %d", tc.height)
				}
			})
		})
	}
}

func TestListViewKeepsFooterPinnedWhenProviderContentIsTall(t *testing.T) {
	m := newModel(appConfig{Providers: []provider{
		{BaseURL: "https://one", Models: []string{"a", "b", "c"}},
		{BaseURL: "https://two", Models: []string{"a", "b", "c"}},
		{BaseURL: "https://three", Models: []string{"a", "b", "c"}},
		{BaseURL: "https://four", Models: []string{"a", "b", "c"}},
		{BaseURL: "https://five", Models: []string{"a", "b", "c"}},
		{BaseURL: "https://six", Models: []string{"a", "b", "c"}},
	}}, "juicy-providers.json")
	m.width = 100

	headerHeight, _, bottomHeight := listLayoutHeightsForTest(m)
	bodyHeight := 10
	m.height = headerHeight + listHeaderGapHeight + bottomHeight + bodyHeight

	view := m.listView()
	lines := strings.Split(view, "\n")
	border := lipgloss.RoundedBorder()

	if got := lipgloss.Height(view); got != m.height {
		t.Fatalf("expected view height %d, got %d", m.height, got)
	}
	if strings.TrimRight(lines[len(lines)-1], " ") != renderShortcutFooter(listModeFooterText()) {
		t.Fatalf("expected footer on last line, got %q", lines[len(lines)-1])
	}
	if strings.TrimRight(lines[len(lines)-2], " ") != m.statusLine() {
		t.Fatalf("expected status on second-to-last line, got %q", lines[len(lines)-2])
	}

	bodyBottomLine := strings.TrimRight(lines[len(lines)-bottomHeight-1], " ")
	if !strings.Contains(bodyBottomLine, border.BottomLeft) || !strings.Contains(bodyBottomLine, border.BottomRight) {
		t.Fatalf("expected pane bottom border directly above bottom bar, got %q", bodyBottomLine)
	}
}

func TestFormViewShowsFieldGuidanceInBothLanguages(t *testing.T) {
	tests := []struct {
		name       string
		lang       appLanguage
		width      int
		title      string
		intro      string
		introParts []string
		keys       string
	}{
		{
			name:  "zh-narrow",
			lang:  langZH,
			width: 80,
			title: "新增供应商",
			intro: "请填写 OAI 兼容 base URL、API key，以及支持逗号或换行的模型列表。",
			introParts: []string{
				"请填写 OAI 兼容 base URL、API",
				"key，以及支持逗号或换行的模型列表。",
			},
			keys: "快捷键：tab/shift+tab 切换焦点 | Ctrl+S 保存 | 模型框 Enter 换行 | Esc 取消 | Ctrl+L 切换中英",
		},
		{
			name:  "zh-wide",
			lang:  langZH,
			width: 140,
			title: "新增供应商",
			intro: "请填写 OAI 兼容 base URL、API key，以及支持逗号或换行的模型列表。",
			introParts: []string{
				"请填写 OAI 兼容 base URL、API",
				"key，以及支持逗号或换行的模型列表。",
			},
			keys: "快捷键：tab/shift+tab 切换焦点 | Ctrl+S 保存 | 模型框 Enter 换行 | Esc 取消 | Ctrl+L 切换中英",
		},
		{
			name:  "en-narrow",
			lang:  langEN,
			width: 80,
			title: "Add Provider",
			intro: "Fill in an OAI-compatible base URL, API key, and models separated by commas or new lines.",
			introParts: []string{
				"Fill in an OAI-compatible base URL,",
				"API key, and models separated",
				"commas or new lines.",
			},
			keys: "Keys: tab/shift+tab move | ctrl+s save | enter adds a new line in Models | esc cancel | ctrl+l toggle lang",
		},
		{
			name:  "en-wide",
			lang:  langEN,
			width: 140,
			title: "Add Provider",
			intro: "Fill in an OAI-compatible base URL, API key, and models separated by commas or new lines.",
			introParts: []string{
				"Fill in an OAI-compatible base URL,",
				"API key, and models separated",
				"commas or new lines.",
			},
			keys: "Keys: tab/shift+tab move | ctrl+s save | enter adds a new line in Models | esc cancel | ctrl+l toggle lang",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(appConfig{}, "juicy-providers.json")
			m.mode = addMode
			m.lang = tc.lang
			m.width = tc.width
			m.applyPlaceholders()

			view := m.View()
			providerTitle := renderPaneTitle(m.tr("供应商", "Providers"))
			resultTitle := renderPaneTitle(m.tr("结果", "Results"))

			if !strings.Contains(view, pageTitleStyle.Render(m.tr("Juicy 批量检测器", "Juicy Batch Checker"))) {
				t.Fatalf("expected shared page header in add-mode view: %q", view)
			}
			if !strings.Contains(view, providerTitle) {
				t.Fatalf("expected provider pane to remain visible in add-mode view: %q", view)
			}
			if !strings.Contains(view, renderPaneTitle(tc.title)) {
				t.Fatalf("expected pane title in form view: %q", view)
			}
			if strings.Contains(view, resultTitle) {
				t.Fatalf("expected results pane title to be replaced in add-mode view: %q", view)
			}
			if !strings.Contains(view, m.tr("还没有保存任何供应商", "No providers saved yet.")) {
				t.Fatalf("expected provider pane content preserved in add-mode view: %q", view)
			}
			if !strings.Contains(view, renderShortcutFooter(tc.keys)) {
				t.Fatalf("expected footer in form view: %q", view)
			}
			assertContainsAll(t, view, tc.introParts...)
			for i, field := range formFields {
				if !strings.Contains(view, renderFieldLabel(field.label.forLang(tc.lang))) {
					t.Fatalf("expected field label %q in view: %q", field.label.forLang(tc.lang), view)
				}
				if !strings.Contains(compactForContains(view), compactForContains(field.helper.forLang(tc.lang))) {
					t.Fatalf("expected field helper %q in view: %q", field.helper.forLang(tc.lang), view)
				}
				placeholder := formFieldPlaceholder(m, i)
				if tc.lang == langZH && placeholder != "" {
					t.Fatalf("expected Chinese placeholder suppressed at index %d, got %q", i, placeholder)
				}
				if tc.lang == langEN && placeholder == "" {
					t.Fatalf("expected English placeholder visible at index %d", i)
				}
			}
		})
	}
}

func TestFormViewRendersMultilineModelsTextareaContent(t *testing.T) {
	m := newModel(appConfig{Providers: []provider{{BaseURL: "https://one", Models: []string{"gpt-4o-mini"}}}}, "juicy-providers.json")
	m.mode = addMode
	m.width = 100
	m.height = 40
	m.modelsInput.SetValue("gpt-4o-mini\nqwen-max\nclaude-3.5-sonnet")
	setFocusedFormField(&m, addProviderModelsField)

	view := m.View()

	assertContainsAll(t, view,
		renderPaneTitle("供应商"),
		renderPaneTitle("新增供应商"),
		selectionStyle.Render("https://one（1 个模型）"),
		"gpt-4o-mini",
		"qwen-max",
		"claude-3.5-sonnet",
	)
}

func TestViewSwitchesOnlyRightPaneInAddMode(t *testing.T) {
	m := newModel(appConfig{Providers: []provider{{BaseURL: "https://one", Models: []string{"gpt-4o-mini"}}}}, "juicy-providers.json")
	m.width = 100
	m.height = 20

	listView := m.View()
	if !strings.Contains(listView, renderPaneTitle("供应商")) {
		t.Fatalf("expected provider pane in list view: %q", listView)
	}
	if !strings.Contains(listView, renderPaneTitle("结果")) {
		t.Fatalf("expected results pane in list view: %q", listView)
	}
	if !strings.Contains(listView, selectionStyle.Render("https://one（1 个模型）")) {
		t.Fatalf("expected provider content in list view: %q", listView)
	}

	updated, cmd := m.Update(keyRunes('a'))
	got := updated.(appModel)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != addMode {
		t.Fatalf("expected add mode, got %v", got.mode)
	}

	addView := got.View()
	if !strings.Contains(addView, renderPaneTitle("供应商")) {
		t.Fatalf("expected provider pane to remain visible in add mode: %q", addView)
	}
	if !strings.Contains(addView, selectionStyle.Render("https://one（1 个模型）")) {
		t.Fatalf("expected provider content to remain visible in add mode: %q", addView)
	}
	if !strings.Contains(addView, renderPaneTitle("新增供应商")) {
		t.Fatalf("expected add-provider pane in add mode: %q", addView)
	}
	if strings.Contains(addView, renderPaneTitle("结果")) {
		t.Fatalf("expected results pane title replaced in add mode: %q", addView)
	}
	assertContainsAll(t, addView,
		renderPaneTitle("新增供应商"),
	)
	if !strings.Contains(addView, "请填写 OAI 兼容 base URL、API") || !strings.Contains(addView, "key，以及支持逗号或换行的模型列表。") {
		t.Fatalf("expected multiline intro in add view: %q", addView)
	}
	if !strings.Contains(addView, renderShortcutFooter(addModeFooterText())) {
		t.Fatalf("expected form shortcuts in add mode footer: %q", addView)
	}
}

func TestFormViewPinsStatusAndFooterToBottomWhenHeightAvailable(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 100
	m.height = 20
	m.applyPlaceholders()

	view := m.View()
	lines := strings.Split(view, "\n")
	bottomHeight := lipgloss.Height(m.formBottomContent())
	border := lipgloss.RoundedBorder()
	footer := renderShortcutFooter(addModeFooterText())

	if got := lipgloss.Height(view); got != m.height {
		t.Fatalf("expected view height %d, got %d", m.height, got)
	}
	if strings.TrimRight(lines[len(lines)-1], " ") != footer {
		t.Fatalf("expected footer on last line, got %q", lines[len(lines)-1])
	}
	if strings.TrimRight(lines[len(lines)-2], " ") != m.statusLine() {
		t.Fatalf("expected status on second-to-last line, got %q", lines[len(lines)-2])
	}
	bodyBottomLine := strings.TrimRight(lines[len(lines)-bottomHeight-1], " ")
	if !strings.Contains(bodyBottomLine, border.BottomLeft) || !strings.Contains(bodyBottomLine, border.BottomRight) {
		t.Fatalf("expected split panes directly above bottom bar, got %q", bodyBottomLine)
	}
	if !strings.Contains(view, renderPaneTitle("供应商")) {
		t.Fatalf("expected provider pane preserved in add mode: %q", view)
	}
	if !strings.Contains(view, renderPaneTitle("新增供应商")) {
		t.Fatalf("expected form pane content preserved: %q", view)
	}
	assertContainsAll(t, view,
		renderPaneTitle("新增供应商"),
	)
	if !strings.Contains(view, "请填写 OAI 兼容 base URL、API") || !strings.Contains(view, "key，以及支持逗号或换行的模型列表。") {
		t.Fatalf("expected multiline intro in pinned view: %q", view)
	}
}

func TestFormViewPinsFooterWithMultilineModelsUnderTightHeight(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 100
	m.height = 20
	m.modelsInput.SetValue("gpt-4o-mini\nqwen-max\nclaude-3.5-sonnet\nllama-3.1-8b\nqvq-plus")
	setFocusedFormField(&m, addProviderModelsField)

	view := m.View()
	lines := strings.Split(view, "\n")
	bottomHeight := lipgloss.Height(m.formBottomContent())
	border := lipgloss.RoundedBorder()
	footer := renderShortcutFooter(addModeFooterText())

	if got := lipgloss.Height(view); got != m.height {
		t.Fatalf("expected view height %d, got %d", m.height, got)
	}
	if strings.TrimRight(lines[len(lines)-1], " ") != footer {
		t.Fatalf("expected footer on last line, got %q", lines[len(lines)-1])
	}
	if strings.TrimRight(lines[len(lines)-2], " ") != m.statusLine() {
		t.Fatalf("expected status on second-to-last line, got %q", lines[len(lines)-2])
	}
	bodyBottomLine := strings.TrimRight(lines[len(lines)-bottomHeight-1], " ")
	if !strings.Contains(bodyBottomLine, border.BottomLeft) || !strings.Contains(bodyBottomLine, border.BottomRight) {
		t.Fatalf("expected split panes directly above bottom bar, got %q", bodyBottomLine)
	}
}

func keyRunes(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func listLayoutHeightsForTest(m appModel) (headerHeight, bodyHeight, bottomHeight int) {
	header := m.renderPageHeaderWithPrompt()
	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderTitledPane(m.tr("供应商", "Providers"), listPaneWidth(m.width), m.providerListView()),
		renderTitledPane(m.tr("结果", "Results"), listPaneWidth(m.width), m.resultListView()),
	)
	bottom := m.listBottomContent()

	return lipgloss.Height(header), lipgloss.Height(body), lipgloss.Height(bottom)
}

func formFieldPlaceholder(m appModel, index int) string {
	switch index {
	case addProviderBaseURLField:
		return m.baseURLInput.Placeholder
	case addProviderAPIKeyField:
		return m.apiKeyInput.Placeholder
	case addProviderModelsField:
		return m.modelsInput.Placeholder
	default:
		return ""
	}
}

func setFocusedFormField(m *appModel, index int) {
	m.baseURLInput.Blur()
	m.apiKeyInput.Blur()
	m.modelsInput.Blur()
	m.focusIndex = index
	m.focusCurrentInput()
}

func assertActiveFormCursorVisible(t *testing.T, m appModel) {
	t.Helper()
	layout := m.formPaneLayout()
	visibleHeight := m.formPaneVisibleContentHeight()
	if visibleHeight <= 0 {
		t.Fatalf("expected positive visible form height, got %d", visibleHeight)
	}
	if layout.activeCursorRow < m.formPaneScrollOffset || layout.activeCursorRow >= m.formPaneScrollOffset+visibleHeight {
		t.Fatalf("expected active cursor row %d inside [%d,%d)", layout.activeCursorRow, m.formPaneScrollOffset, m.formPaneScrollOffset+visibleHeight)
	}
}

func thumbRows(lines []string) []int {
	rows := make([]int, 0)
	for i, line := range lines {
		if lastRuneString(line) == "▌" {
			rows = append(rows, i)
		}
	}
	return rows
}

func listModeFooterText() string {
	return "快捷键：Tab 编辑提示词 | a 新增供应商 | e 编辑供应商 | Enter 开始检测 | j/k 移动 | l 切换中英 | q 退出"
}

func addModeFooterText() string {
	return "快捷键：tab/shift+tab 切换焦点 | Ctrl+S 保存 | 模型框 Enter 换行 | Esc 取消 | Ctrl+L 切换中英"
}

func editModeFooterText() string {
	return "快捷键：tab/shift+tab 切换焦点 | Ctrl+S 更新 | 模型框 Enter 换行 | Esc 取消编辑 | Ctrl+L 切换中英"
}
