package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestDebugOutputResetAndLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.log")
	if err := os.WriteFile(path, []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("seed debug file: %v", err)
	}

	if err := resetDebugOutputFile(path); err != nil {
		t.Fatalf("reset debug file: %v", err)
	}

	logger := newDebugOutput(path)
	if err := logger.log("window_size", debugInt("terminal_width", 80), debugInt("terminal_height", 24)); err != nil {
		t.Fatalf("log debug line: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read debug file: %v", err)
	}

	if got, want := strings.TrimSpace(string(data)), "event=window_size terminal_width=80 terminal_height=24"; got != want {
		t.Fatalf("unexpected debug line:\nwant %q\ngot  %q", want, got)
	}
}

func TestDebugOutputSkipsConsecutiveDuplicateLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.log")
	if err := resetDebugOutputFile(path); err != nil {
		t.Fatalf("reset debug file: %v", err)
	}

	logger := newDebugOutput(path)
	if err := logger.log("list_layout", debugInt("terminal_height", 24), debugInt("body_height", 10)); err != nil {
		t.Fatalf("first log: %v", err)
	}
	if err := logger.log("list_layout", debugInt("terminal_height", 24), debugInt("body_height", 10)); err != nil {
		t.Fatalf("duplicate log: %v", err)
	}
	if err := logger.log("list_layout", debugInt("terminal_height", 24), debugInt("body_height", 11)); err != nil {
		t.Fatalf("changed log: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read debug file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if got := len(lines); got != 2 {
		t.Fatalf("expected 2 debug lines after dedupe, got %d: %q", got, string(data))
	}
	if lines[0] != "event=list_layout terminal_height=24 body_height=10" {
		t.Fatalf("unexpected first debug line: %q", lines[0])
	}
	if lines[1] != "event=list_layout terminal_height=24 body_height=11" {
		t.Fatalf("unexpected second debug line: %q", lines[1])
	}
}

func TestWindowSizeMsgLogsTerminalDimensions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window-size.log")
	if err := resetDebugOutputFile(path); err != nil {
		t.Fatalf("reset debug file: %v", err)
	}

	m := newModel(appConfig{}, "juicy-providers.json")
	m.debug = newDebugOutput(path)

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	got := updated.(appModel)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.width != 90 || got.height != 24 {
		t.Fatalf("expected updated size 90x24, got %dx%d", got.width, got.height)
	}

	fields := readSingleDebugLine(t, path)
	if fields["event"] != "window_size" {
		t.Fatalf("expected window_size event, got %q", fields["event"])
	}
	if fields["terminal_width"] != "90" || fields["terminal_height"] != "24" {
		t.Fatalf("unexpected terminal dimensions: %+v", fields)
	}
}

func TestListViewLogsClampedLayoutNumbersForSmallHeight(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list-layout.log")
	if err := resetDebugOutputFile(path); err != nil {
		t.Fatalf("reset debug file: %v", err)
	}

	m := newModel(appConfig{}, "juicy-providers.json")
	m.debug = newDebugOutput(path)
	m.width = 100
	m.height = 1

	header := m.renderPageHeader(
		m.tr("Juicy 批量检测器", "Juicy Batch Checker"),
		m.tr("提示词：", "Prompt: ")+juicyPrompt,
	)
	bottom := m.listBottomContent()
	bodyHeightBudget := m.availableListBodyHeight(header, bottom)
	providerPane := renderTitledPaneWithHeight(m.tr("供应商", "Providers"), listPaneWidth(m.width), bodyHeightBudget, m.providerListView())
	resultPane := renderTitledPaneWithHeight(m.tr("结果", "Results"), listPaneWidth(m.width), bodyHeightBudget, m.resultListView())
	body := lipgloss.JoinHorizontal(lipgloss.Top, providerPane, resultPane)
	mainContent := lipgloss.JoinVertical(lipgloss.Left, header, "", body)

	view := m.listView()
	if view == "" {
		t.Fatal("expected rendered list view")
	}

	fields := readSingleDebugLine(t, path)
	assertDebugIntField(t, fields, "terminal_width", m.width)
	assertDebugIntField(t, fields, "terminal_height", m.height)
	assertDebugIntField(t, fields, "header_height", lipgloss.Height(header))
	assertDebugIntField(t, fields, "bottom_bar_height", lipgloss.Height(bottom))
	assertDebugIntField(t, fields, "body_height_budget", 0)
	assertDebugIntField(t, fields, "provider_pane_height", lipgloss.Height(providerPane))
	assertDebugIntField(t, fields, "result_pane_height", lipgloss.Height(resultPane))
	assertDebugIntField(t, fields, "body_height", lipgloss.Height(body))
	assertDebugIntField(t, fields, "main_content_height", lipgloss.Height(mainContent))
	assertDebugIntField(t, fields, "total_view_height", lipgloss.Height(view))
}

func TestFormViewLogsLayoutNumbers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "form-layout.log")
	if err := resetDebugOutputFile(path); err != nil {
		t.Fatalf("reset debug file: %v", err)
	}

	m := newModel(appConfig{}, "juicy-providers.json")
	m.debug = newDebugOutput(path)
	m.mode = addMode
	m.width = 100
	m.height = 20
	m.applyPlaceholders()

	paneWidth := formPaneWidth(m.width)
	sections := []string{
		helperTextStyle.Render(m.tr("请填写 OAI 兼容 base URL、API key 和模型列表（逗号分隔）。", "Fill in an OAI-compatible base URL, API key, and comma-separated models.")),
	}
	for i, field := range formFields {
		sections = append(sections, m.renderFormField(field, m.inputs[i].View()))
	}
	formPane := renderTitledPane(m.tr("新增供应商", "Add Provider"), paneWidth, strings.Join(sections, "\n\n"))
	bottom := lipgloss.JoinVertical(lipgloss.Left,
		m.statusLine(),
		renderShortcutFooter(m.tr("快捷键：tab/shift+tab 切换焦点 | Enter 保存 | Esc 取消 | l 切换中英", "Keys: tab/shift+tab move | enter save | esc cancel | l toggle lang")),
	)

	view := m.formView()
	if view == "" {
		t.Fatal("expected rendered form view")
	}

	fields := readSingleDebugLine(t, path)
	if fields["event"] != "form_layout" {
		t.Fatalf("expected form_layout event, got %q", fields["event"])
	}
	assertDebugIntField(t, fields, "terminal_width", m.width)
	assertDebugIntField(t, fields, "terminal_height", m.height)
	assertDebugIntField(t, fields, "form_pane_width", paneWidth)
	assertDebugIntField(t, fields, "form_pane_height", lipgloss.Height(formPane))
	assertDebugIntField(t, fields, "bottom_bar_height", lipgloss.Height(bottom))
	assertDebugIntField(t, fields, "main_content_height", lipgloss.Height(formPane))
	assertDebugIntField(t, fields, "total_view_height", lipgloss.Height(view))
}

func readSingleDebugLine(t *testing.T, path string) map[string]string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read debug file: %v", err)
	}

	text := strings.TrimSpace(string(data))
	if text == "" {
		t.Fatal("expected debug file to contain one line")
	}

	lines := strings.Split(text, "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one debug line, got %d: %q", len(lines), text)
	}

	fields := make(map[string]string, len(strings.Fields(lines[0])))
	for _, part := range strings.Fields(lines[0]) {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			t.Fatalf("invalid debug field %q", part)
		}
		fields[key] = value
	}

	return fields
}

func assertDebugIntField(t *testing.T, fields map[string]string, key string, want int) {
	t.Helper()

	got, ok := fields[key]
	if !ok {
		t.Fatalf("missing debug field %q in %+v", key, fields)
	}

	parsed, err := strconv.Atoi(got)
	if err != nil {
		t.Fatalf("parse debug field %q=%q: %v", key, got, err)
	}
	if parsed != want {
		t.Fatalf("unexpected %s: want %d, got %d", key, want, parsed)
	}
}
