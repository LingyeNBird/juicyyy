package main

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type localizedText struct {
	zh string
	en string
}

type formFieldSpec struct {
	label       localizedText
	placeholder localizedText
	helper      localizedText
	kind        inputKind
}

var formFields = []formFieldSpec{
	{
		label:       localizedText{zh: "基础 URL", en: "Base URL"},
		placeholder: localizedText{zh: "OAI 基础 URL 或 /chat/completions URL", en: "OAI base URL or /chat/completions URL"},
		helper:      localizedText{zh: "支持基础 URL，或直接粘贴 /chat/completions 地址。", en: "Use a base URL or paste a full /chat/completions endpoint."},
		kind:        inputKindText,
	},
	{
		label:       localizedText{zh: "API 密钥", en: "API Key"},
		placeholder: localizedText{zh: "API 密钥", en: "API key"},
		helper:      localizedText{zh: "密钥会以密码模式隐藏显示。", en: "The key is masked while you type."},
		kind:        inputKindPassword,
	},
	{
		label:       localizedText{zh: "模型列表", en: "Models"},
		placeholder: localizedText{zh: "模型列表（逗号分隔）", en: "Models (comma separated)"},
		helper:      localizedText{zh: "用逗号分隔多个模型名称，不限制长度。", en: "Separate multiple model names with commas; no length limit."},
		kind:        inputKindModels,
	},
}

func (t localizedText) forLang(lang appLanguage) string {
	if lang == langEN {
		return t.en
	}
	return t.zh
}

func (m appModel) tr(zh, en string) string {
	if m.lang == langEN {
		return en
	}
	return zh
}

func (m *appModel) applyPlaceholders() {
	paneWidth := formPaneWidth(m.width)
	if m.mode == addMode {
		paneWidth = listPaneWidth(m.width)
	}
	applyInputLocale(m.inputs, m.lang, paneWidth)
}

func applyInputLocale(inputs []textinput.Model, lang appLanguage, paneWidth int) {
	inputWidth := inputWidthForFormPane(paneWidth)
	for i := range inputs {
		inputs[i].Placeholder = safePlaceholder(formFields[i].placeholder.forLang(lang))
		inputs[i].Width = inputWidth
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
		m.setStatus(statusInfo, "Language switched to English.")
	} else {
		m.lang = langZH
		m.setStatus(statusInfo, "语言已切换为中文。")
	}
	m.applyPlaceholders()
}
