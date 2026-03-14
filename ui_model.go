package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	rw "github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

type appLanguage int

type statusKind int

type inputKind int

const (
	langZH appLanguage = iota
	langEN

	statusInfo statusKind = iota
	statusSuccess
	statusError
	statusWarning
	statusLoading

	inputKindText inputKind = iota
	inputKindPassword
	inputKindModels

	defaultInputCharLimit = 512
	minListPaneWidth      = 36
	minFormPaneWidth      = 72
	layoutOuterPadding    = 6
	listHeaderGapHeight   = 1
	paneVerticalChrome    = 4
	formInputWidthSlack   = 16
	minInputWidth         = 24
	modelsInputPrompt     = "> "
	modelsInputIndent     = "  "
)

const (
	addProviderBaseURLField = iota
	addProviderAPIKeyField
	addProviderModelsField
	addProviderFieldCount
	noEditingProviderIndex = -1
)

type viewMode int

const (
	listMode viewMode = iota
	addMode
)

type appModel struct {
	config        appConfig
	configPath    string
	lang          appLanguage
	mode          viewMode
	editingIndex  int
	promptInput   textinput.Model
	promptEditing bool
	cursor        int
	activeResult  int
	baseURLInput  textinput.Model
	apiKeyInput   textinput.Model
	modelsInput   textarea.Model
	focusIndex    int
	width         int
	height        int
	status        string
	statusKind    statusKind
	results       []modelResult
	running       bool
	spinner       spinner.Model
	concurrency   int
}

func newModel(cfg appConfig, configPath string) appModel {
	spin := spinner.New()
	spin.Spinner = spinner.Line
	spin.Style = loadingStyle

	baseURLInput, apiKeyInput, modelsInput := newProviderInputs(langZH)
	promptInput := newPromptInput()
	m := appModel{
		config:       cfg,
		configPath:   configPath,
		lang:         langZH,
		mode:         listMode,
		editingIndex: noEditingProviderIndex,
		promptInput:  promptInput,
		baseURLInput: baseURLInput,
		apiKeyInput:  apiKeyInput,
		modelsInput:  modelsInput,
		spinner:      spin,
		concurrency:  5,
	}
	m.setStatus(statusInfo, fmt.Sprintf("配置文件：%s", configPath))
	return m
}

func (m *appModel) setStatus(kind statusKind, text string) {
	m.statusKind = kind
	m.status = text
}

func listPaneWidth(totalWidth int) int {
	return maxInt(minListPaneWidth, totalWidth/2-layoutOuterPadding/2)
}

func formPaneWidth(totalWidth int) int {
	return maxInt(minFormPaneWidth, totalWidth-layoutOuterPadding)
}

func (m appModel) availableListBodyHeight(header, bottomContent string) int {
	if m.height <= 0 {
		return 0
	}

	return maxInt(0, m.height-lipgloss.Height(header)-listHeaderGapHeight-lipgloss.Height(bottomContent))
}

func inputWidthForFormPane(paneWidth int) int {
	return maxInt(minInputWidth, paneWidth-formInputWidthSlack)
}

func paneContentWidth(paneWidth int) int {
	return maxInt(1, paneWidth-paneStyle.GetPaddingLeft()-paneStyle.GetPaddingRight())
}

func modelsInputWidthForPane(paneWidth int) int {
	return paneContentWidth(paneWidth)
}

func modelsInputTextWidthForPane(paneWidth int) int {
	return maxInt(1, modelsInputWidthForPane(paneWidth)-lipgloss.Width(modelsInputPrompt))
}

func modelsInputHeightForValue(value string, paneWidth int) int {
	return maxInt(1, wrappedVisibleRowCount(value, modelsInputTextWidthForPane(paneWidth)))
}

func (m appModel) activeFormPaneWidth() int {
	if m.mode == addMode {
		return listPaneWidth(m.width)
	}
	return formPaneWidth(m.width)
}

func (m appModel) isEditingProvider() bool {
	return m.editingIndex >= 0 && m.editingIndex < len(m.config.Providers)
}

func wrappedVisibleRowCount(value string, width int) int {
	if width <= 0 {
		return 1
	}

	rows := 0
	for _, line := range strings.Split(value, "\n") {
		rows += len(wrapTextareaLine([]rune(line), width))
	}

	return maxInt(1, rows)
}

// wrapTextareaLine mirrors bubbles/textarea soft wrapping so height matches the
// rows that textarea.Model will actually render.
func wrapTextareaLine(runes []rune, width int) [][]rune {
	if width <= 0 {
		return [][]rune{{}}
	}

	var (
		lines  = [][]rune{{}}
		word   []rune
		row    int
		spaces int
	)

	for _, r := range runes {
		if unicode.IsSpace(r) {
			spaces++
		} else {
			word = append(word, r)
		}

		if spaces > 0 {
			if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces > width {
				row++
				lines = append(lines, []rune{})
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatedSpaces(spaces)...)
				spaces = 0
				word = nil
			} else {
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatedSpaces(spaces)...)
				spaces = 0
				word = nil
			}
		} else {
			lastCharLen := rw.RuneWidth(word[len(word)-1])
			if uniseg.StringWidth(string(word))+lastCharLen > width {
				if len(lines[row]) > 0 {
					row++
					lines = append(lines, []rune{})
				}
				lines[row] = append(lines[row], word...)
				word = nil
			}
		}
	}

	if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces >= width {
		lines = append(lines, []rune{})
		lines[row+1] = append(lines[row+1], word...)
		spaces++
		lines[row+1] = append(lines[row+1], repeatedSpaces(spaces)...)
	} else {
		lines[row] = append(lines[row], word...)
		spaces++
		lines[row] = append(lines[row], repeatedSpaces(spaces)...)
	}

	return lines
}

func repeatedSpaces(n int) []rune {
	return []rune(strings.Repeat(" ", n))
}

func promptInputWidth(totalWidth int, label string) int {
	return maxInt(minInputWidth, totalWidth-layoutOuterPadding-lipgloss.Width(label)-1)
}
