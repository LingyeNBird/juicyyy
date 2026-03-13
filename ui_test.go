package main

import "testing"

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}

func TestFormViewWithChinesePlaceholdersDoesNotPanic(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 120

	assertNoPanic(t, func() {
		_ = m.formView()
	})
}

func TestLanguageToggleInAddModeDoesNotPanic(t *testing.T) {
	m := newModel(appConfig{}, "juicy-providers.json")
	m.mode = addMode
	m.width = 120

	m.toggleLanguage()
	assertNoPanic(t, func() {
		_ = m.formView()
	})

	m.toggleLanguage()
	assertNoPanic(t, func() {
		_ = m.formView()
	})
}

func TestApplyInputLocaleAdjustsWidthForWidePlaceholders(t *testing.T) {
	inputs := newInputs(langEN)
	for i := range inputs {
		if inputs[i].Width != defaultInputWidth {
			t.Fatalf("expected English input width %d, got %d at index %d", defaultInputWidth, inputs[i].Width, i)
		}
	}

	applyInputLocale(inputs, langZH)
	for i := range inputs {
		if inputs[i].Width != defaultInputWidth {
			t.Fatalf("expected Chinese input width %d, got %d at index %d", defaultInputWidth, inputs[i].Width, i)
		}
		if inputs[i].Placeholder != "" {
			t.Fatalf("expected Chinese placeholder to be suppressed for safety, got %q at index %d", inputs[i].Placeholder, i)
		}
	}
}

func TestModelsInputHasNoCharLimit(t *testing.T) {
	inputs := newInputs(langEN)

	if inputs[0].CharLimit != defaultInputCharLimit {
		t.Fatalf("expected Base URL char limit %d, got %d", defaultInputCharLimit, inputs[0].CharLimit)
	}
	if inputs[1].CharLimit != defaultInputCharLimit {
		t.Fatalf("expected API key char limit %d, got %d", defaultInputCharLimit, inputs[1].CharLimit)
	}
	if inputs[2].CharLimit != 0 {
		t.Fatalf("expected Models char limit 0 (unlimited), got %d", inputs[2].CharLimit)
	}
}
