package main

import (
	"github.com/charmbracelet/bubbles/textarea"
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
		placeholder: localizedText{zh: "每行或逗号分隔一个模型", en: "One model per line or comma separated"},
		helper:      localizedText{zh: "支持逗号或换行分隔多个模型名称，可直接粘贴多行列表。", en: "Use commas or new lines for multiple model names; paste multi-line lists directly."},
		kind:        inputKindModels,
	},
}

var requestSettingsFields = []formFieldSpec{
	{
		label:       localizedText{zh: "请求提示词", en: "Request Prompt"},
		placeholder: localizedText{zh: "输入发送给模型的提示词", en: "Enter the prompt sent to the model"},
		helper:      localizedText{zh: "检测时会将这段文本直接作为用户请求发送。", en: "This text is sent directly as the user request during checks."},
		kind:        inputKindText,
	},
	{
		label:       localizedText{zh: "请求时间间隔(s)", en: "Request Interval (s)"},
		placeholder: localizedText{zh: "0", en: "0"},
		helper:      localizedText{zh: "相邻请求之间等待的秒数，支持小数；默认 0。", en: "Seconds to wait between outgoing requests; decimals allowed, defaults to 0."},
		kind:        inputKindText,
	},
	{
		label:       localizedText{zh: "超时时间(s)", en: "Timeout (s)"},
		placeholder: localizedText{zh: "180", en: "180"},
		helper:      localizedText{zh: "单次 HTTP 请求超时，默认 180 秒。", en: "Per-request HTTP timeout in seconds; defaults to 180."},
		kind:        inputKindText,
	},
	{
		label:       localizedText{zh: "请求方式", en: "Request Mode"},
		placeholder: localizedText{zh: "", en: ""},
		helper:      localizedText{zh: "在 ChatGPT Responses API 和 OpenAI 兼容 /chat/completions 之间切换。", en: "Switch between ChatGPT Responses API and OpenAI-compatible /chat/completions."},
		kind:        inputKindText,
	},
	{
		label:       localizedText{zh: "重试次数", en: "Retries"},
		placeholder: localizedText{zh: "5", en: "5"},
		helper:      localizedText{zh: "当结果为 0 或非数字时额外重试的次数。", en: "Extra retry attempts when the result is 0 or non-numeric."},
		kind:        inputKindText,
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
	applyFormLocale(&m.baseURLInput, &m.apiKeyInput, &m.modelsInput, m.lang, m.activeFormPaneWidth())
	applyRequestSettingsLocale(&m.requestPromptInput, &m.requestIntervalInput, &m.requestTimeoutInput, &m.requestRetryInput, m.lang, m.activeFormPaneWidth())
}

func applyFormLocale(baseURLInput, apiKeyInput *textinput.Model, modelsInput *textarea.Model, lang appLanguage, paneWidth int) {
	inputWidth := inputWidthForFormPane(paneWidth)
	applyTextInputLocale(baseURLInput, formFields[addProviderBaseURLField], lang, inputWidth)
	applyTextInputLocale(apiKeyInput, formFields[addProviderAPIKeyField], lang, inputWidth)
	applyTextareaLocale(modelsInput, formFields[addProviderModelsField], lang)
	syncModelsInputLayout(modelsInput, paneWidth)
}

func applyRequestSettingsLocale(promptInput, intervalInput, timeoutInput, retryInput *textinput.Model, lang appLanguage, paneWidth int) {
	inputWidth := inputWidthForFormPane(paneWidth)
	applyTextInputLocale(promptInput, requestSettingsFields[requestSettingsPromptField], lang, inputWidth)
	applyTextInputLocale(intervalInput, requestSettingsFields[requestSettingsIntervalField], lang, inputWidth)
	applyTextInputLocale(timeoutInput, requestSettingsFields[requestSettingsTimeoutField], lang, inputWidth)
	applyTextInputLocale(retryInput, requestSettingsFields[requestSettingsRetryField], lang, inputWidth)
}

func applyTextInputLocale(input *textinput.Model, field formFieldSpec, lang appLanguage, inputWidth int) {
	input.Placeholder = safePlaceholder(field.placeholder.forLang(lang))
	input.Width = inputWidth
}

func applyTextareaLocale(input *textarea.Model, field formFieldSpec, lang appLanguage) {
	input.Placeholder = safePlaceholder(field.placeholder.forLang(lang))
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
