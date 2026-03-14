package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
)

type appLanguage int

const (
	langZH appLanguage = iota
	langEN
	defaultInputWidth     = 56
	defaultInputCharLimit = 512
)

type viewMode int

const (
	listMode viewMode = iota
	addMode
)

type appModel struct {
	config       appConfig
	configPath   string
	lang         appLanguage
	mode         viewMode
	cursor       int
	activeResult int
	inputs       []textinput.Model
	focusIndex   int
	width        int
	height       int
	status       string
	results      []modelResult
	running      bool
	spinner      spinner.Model
	concurrency  int
}

func newModel(cfg appConfig, configPath string) appModel {
	spin := spinner.New()
	spin.Spinner = spinner.Line
	spin.Style = goodStyle

	inputs := newInputs(langZH)

	return appModel{
		config:      cfg,
		configPath:  configPath,
		lang:        langZH,
		mode:        listMode,
		inputs:      inputs,
		status:      fmt.Sprintf("配置文件：%s", configPath),
		spinner:     spin,
		concurrency: 5,
	}
}
