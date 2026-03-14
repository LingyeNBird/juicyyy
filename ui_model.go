package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
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
)

type viewMode int

const (
	listMode viewMode = iota
	addMode
)

type appModel struct {
	config       appConfig
	configPath   string
	debug        *debugOutput
	lang         appLanguage
	mode         viewMode
	cursor       int
	activeResult int
	inputs       []textinput.Model
	focusIndex   int
	width        int
	height       int
	status       string
	statusKind   statusKind
	results      []modelResult
	running      bool
	spinner      spinner.Model
	concurrency  int
}

func newModel(cfg appConfig, configPath string) appModel {
	spin := spinner.New()
	spin.Spinner = spinner.Line
	spin.Style = loadingStyle

	inputs := newInputs(langZH)
	m := appModel{
		config:      cfg,
		configPath:  configPath,
		debug:       activeDebugOutput,
		lang:        langZH,
		mode:        listMode,
		inputs:      inputs,
		spinner:     spin,
		concurrency: 5,
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
