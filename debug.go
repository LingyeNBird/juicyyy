package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const runtimeDebugFilePath = "juicy-debug.log"

var activeDebugOutput *debugOutput

type debugOutput struct {
	path     string
	mu       sync.Mutex
	lastLine string
}

type debugField struct {
	key   string
	value int
}

func newDebugOutput(path string) *debugOutput {
	return &debugOutput{path: path}
}

func resetDebugOutputFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, nil, 0o600)
}

func debugInt(key string, value int) debugField {
	return debugField{key: key, value: value}
}

func (d *debugOutput) log(event string, fields ...debugField) error {
	if d == nil {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	line := d.line(event, fields...)
	if line == d.lastLine {
		return nil
	}
	file, err := os.OpenFile(d.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err = file.WriteString(line + "\n"); err != nil {
		return err
	}
	d.lastLine = line
	return nil
}

func (d *debugOutput) line(event string, fields ...debugField) string {
	parts := make([]string, 0, len(fields)+1)
	parts = append(parts, fmt.Sprintf("event=%s", event))
	for _, field := range fields {
		parts = append(parts, fmt.Sprintf("%s=%d", field.key, field.value))
	}
	return strings.Join(parts, " ")
}

func (m appModel) logWindowSize() {
	_ = m.debug.log("window_size",
		debugInt("terminal_width", m.width),
		debugInt("terminal_height", m.height),
	)
}

func (m appModel) logListLayout(headerHeight, bottomBarHeight, bodyHeightBudget, providerPaneHeight, resultPaneHeight, bodyHeight, mainContentHeight, totalViewHeight int) {
	_ = m.debug.log("list_layout",
		debugInt("terminal_width", m.width),
		debugInt("terminal_height", m.height),
		debugInt("header_height", headerHeight),
		debugInt("bottom_bar_height", bottomBarHeight),
		debugInt("body_height_budget", bodyHeightBudget),
		debugInt("provider_pane_height", providerPaneHeight),
		debugInt("result_pane_height", resultPaneHeight),
		debugInt("body_height", bodyHeight),
		debugInt("main_content_height", mainContentHeight),
		debugInt("total_view_height", totalViewHeight),
	)
}

func (m appModel) logFormLayout(formPaneWidth, formPaneHeight, bottomBarHeight, mainContentHeight, totalViewHeight int) {
	_ = m.debug.log("form_layout",
		debugInt("terminal_width", m.width),
		debugInt("terminal_height", m.height),
		debugInt("form_pane_width", formPaneWidth),
		debugInt("form_pane_height", formPaneHeight),
		debugInt("bottom_bar_height", bottomBarHeight),
		debugInt("main_content_height", mainContentHeight),
		debugInt("total_view_height", totalViewHeight),
	)
}
