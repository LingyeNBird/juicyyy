package main

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

type runFinishedMsg struct {
	Results []modelResult
}

type runProgressMsg struct {
	Completed int
	Total     int
}

func startRunChecks(selected provider, settings requestSettings, concurrency int) <-chan tea.Msg {
	events := make(chan tea.Msg, maxInt(1, len(selected.Models)+1))
	go func() {
		results := runJuicyChecks(context.Background(), selected, settings, concurrency, func(completed, total int) {
			events <- runProgressMsg{Completed: completed, Total: total}
		})
		events <- runFinishedMsg{Results: results}
		close(events)
	}()
	return events
}

func waitForRunMsgCmd(events <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-events
		if !ok {
			return nil
		}
		return msg
	}
}

func runChecksCmd(events <-chan tea.Msg) tea.Cmd {
	return waitForRunMsgCmd(events)
}
