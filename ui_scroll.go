package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type paneScrollDirection int

const (
	paneScrollDirectionNeutral paneScrollDirection = iota
	paneScrollDirectionUp
	paneScrollDirectionDown
)

const paneScrollLeadRows = 2

type paneContentLayout struct {
	body            string
	wrappedLines    []string
	activeCursorRow int
	activeEndRow    int
}

func syncPaneScrollOffset(layout paneContentLayout, visibleHeight, currentOffset int, direction paneScrollDirection, anchorRow int) int {
	if visibleHeight <= 0 {
		return 0
	}

	maxOffset := maxInt(0, len(layout.wrappedLines)-visibleHeight)
	offset := maxInt(0, minInt(currentOffset, maxOffset))
	if layout.activeCursorRow < 0 || len(layout.wrappedLines) == 0 {
		return offset
	}

	activeCursorRow := maxInt(0, minInt(layout.activeCursorRow, len(layout.wrappedLines)-1))
	activeEndRow := maxInt(activeCursorRow, minInt(layout.activeEndRow, len(layout.wrappedLines)-1))
	anchorRow = maxInt(0, minInt(anchorRow, len(layout.wrappedLines)-1))
	margin := paneScrollMargin(visibleHeight)

	switch direction {
	case paneScrollDirectionUp:
		topThreshold := offset + margin
		if anchorRow < topThreshold {
			offset = anchorRow - margin
		}
	case paneScrollDirectionDown:
		bottomThreshold := offset + visibleHeight - 1 - margin
		if activeEndRow > bottomThreshold {
			offset = activeEndRow - (visibleHeight - 1 - margin)
		}
	}

	if direction == paneScrollDirectionUp && anchorRow < offset {
		offset = anchorRow
	}
	if activeCursorRow < offset {
		offset = activeCursorRow
	}
	if activeEndRow >= offset+visibleHeight {
		offset = activeEndRow - visibleHeight + 1
	}

	return maxInt(0, minInt(offset, maxOffset))
}

func paneScrollMargin(visibleHeight int) int {
	if visibleHeight <= paneScrollLeadRows*2+1 {
		return 0
	}
	return paneScrollLeadRows
}

func paneScrollDirectionForKey(key string) paneScrollDirection {
	switch key {
	case "up", "shift+tab":
		return paneScrollDirectionUp
	case "down", "tab":
		return paneScrollDirectionDown
	default:
		return paneScrollDirectionNeutral
	}
}

func (m *appModel) syncProviderPaneScroll(direction paneScrollDirection) {
	layout := m.providerPaneLayout()
	visibleHeight := m.splitPaneVisibleContentHeight(m.currentSplitPaneBottomContent())
	anchorRow := layout.activeCursorRow
	m.providerPaneScrollOffset = syncPaneScrollOffset(layout, visibleHeight, m.providerPaneScrollOffset, direction, anchorRow)
}

func (m *appModel) syncResultsPaneScroll() {
	layout := m.resultPaneLayout()
	visibleHeight := m.splitPaneVisibleContentHeight(m.listBottomContent())
	anchorRow := layout.activeCursorRow
	m.resultsPaneScrollOffset = syncPaneScrollOffset(layout, visibleHeight, m.resultsPaneScrollOffset, paneScrollDirectionNeutral, anchorRow)
}

func (m *appModel) syncVisiblePaneScrolls() {
	m.syncProviderPaneScroll(paneScrollDirectionNeutral)
	if m.mode == addMode {
		m.syncFormPaneScroll()
		return
	}
	m.syncResultsPaneScroll()
}

func (m appModel) currentSplitPaneBottomContent() string {
	if m.mode == addMode {
		return m.formBottomContent()
	}
	return m.listBottomContent()
}

func (m *appModel) focusListPane(focus listPaneFocus) {
	m.listPaneFocus = focus
}

func (m *appModel) scrollFocusedListPane(delta int) {
	if m.mode != listMode || delta == 0 {
		return
	}

	visibleHeight := m.splitPaneVisibleContentHeight(m.listBottomContent())
	if m.listPaneFocus == resultsPaneFocus {
		m.resultsPaneScrollOffset = clampPaneScrollOffset(m.resultPaneLayout(), visibleHeight, m.resultsPaneScrollOffset+delta)
		return
	}
	m.providerPaneScrollOffset = clampPaneScrollOffset(m.providerPaneLayout(), visibleHeight, m.providerPaneScrollOffset+delta)
}

func clampPaneScrollOffset(layout paneContentLayout, visibleHeight, offset int) int {
	if visibleHeight <= 0 {
		return 0
	}
	maxOffset := maxInt(0, len(layout.wrappedLines)-visibleHeight)
	return maxInt(0, minInt(offset, maxOffset))
}

func (m appModel) listPaneBounds() (providerBounds, resultsBounds paneBounds) {
	header := m.renderPageHeaderWithPrompt()
	paneWidth := listPaneWidth(m.width)
	bodyTop := lipgloss.Height(header) + 1
	bodyHeight := m.availableListBodyHeight(header, m.listBottomContent())

	providerBounds = paneBounds{x: 0, y: bodyTop, width: paneWidth, height: bodyHeight}
	resultsBounds = paneBounds{x: paneWidth, y: bodyTop, width: paneWidth, height: bodyHeight}
	return providerBounds, resultsBounds
}

type paneBounds struct {
	x      int
	y      int
	width  int
	height int
}

func (b paneBounds) contains(x, y int) bool {
	if b.width <= 0 || b.height <= 0 {
		return false
	}
	return x >= b.x && x < b.x+b.width && y >= b.y && y < b.y+b.height
}

func mouseEventKind(msg tea.MouseMsg) string {
	return msg.String()
}
