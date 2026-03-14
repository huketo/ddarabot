package translator

import (
	"strings"
	"testing"
)

func TestBuildTranslateSystemPrompt(t *testing.T) {
	prompt := buildTranslateSystemPrompt("ko", "en")
	if !strings.Contains(prompt, "ko") || !strings.Contains(prompt, "en") {
		t.Errorf("prompt missing language codes: %s", prompt)
	}
	if !strings.Contains(prompt, "do not translate hashtags") {
		t.Error("prompt missing hashtag rule")
	}
}

func TestBuildSummarizeSystemPrompt(t *testing.T) {
	prompt := buildSummarizeSystemPrompt("ko", "en", 250)
	if !strings.Contains(prompt, "250") {
		t.Errorf("prompt missing max graphemes: %s", prompt)
	}
}
