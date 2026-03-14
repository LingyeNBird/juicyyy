package main

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

func inputPlaceholders(lang appLanguage) []string {
	if lang == langEN {
		return []string{
			"OAI base URL or /chat/completions URL",
			"API key",
			"Models (comma separated)",
		}
	}

	return []string{
		"OAI 基础 URL 或 /chat/completions URL",
		"API 密钥",
		"模型列表（逗号分隔）",
	}
}

func (m appModel) tr(zh, en string) string {
	if m.lang == langEN {
		return en
	}
	return zh
}

func (m *appModel) applyPlaceholders() {
	applyInputLocale(m.inputs, m.lang)
}

func applyInputLocale(inputs []textinput.Model, lang appLanguage) {
	placeholders := inputPlaceholders(lang)
	for i := range inputs {
		inputs[i].Placeholder = safePlaceholder(placeholders[i])
		inputs[i].Width = defaultInputWidth
	}
}

func placeholderHasWideRunes(text string) bool {
	return lipgloss.Width(text) > len([]rune(text))
}

func safePlaceholder(text string) string {
	if placeholderHasWideRunes(text) {
		return ""
	}
	return text
}

func (m *appModel) toggleLanguage() {
	if m.lang == langZH {
		m.lang = langEN
		m.status = "Language switched to English."
	} else {
		m.lang = langZH
		m.status = "语言已切换为中文。"
	}
	m.applyPlaceholders()
}
