package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	configPath := filepath.Join(".", "juicy-providers.json")
	activeDebugOutput = newDebugOutput(runtimeDebugFilePath)
	if err := resetDebugOutputFile(runtimeDebugFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "reset debug file: %v\n", err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	program := tea.NewProgram(newModel(cfg, configPath), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run app: %v\n", err)
		os.Exit(1)
	}
}
