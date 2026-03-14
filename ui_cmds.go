package main

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

type runFinishedMsg struct {
	Results []modelResult
}

func runChecksCmd(selected provider, prompt string, concurrency int) tea.Cmd {
	return func() tea.Msg {
		results := runJuicyChecks(context.Background(), selected, prompt, concurrency)
		return runFinishedMsg{Results: results}
	}
}
