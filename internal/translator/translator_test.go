package translator

import (
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

func TestCountGraphemes(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"hello", 5},
		{"한국어", 3},
		{"hello 🌍", 7},
	}

	for _, tt := range tests {
		got := uniseg.GraphemeClusterCount(tt.text)
		if got != tt.want {
			t.Errorf("GraphemeClusterCount(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestNeedsOverflowSummary(t *testing.T) {
	footer := "\n\n🌐 Translated by #DDaraBot"
	footerLen := uniseg.GraphemeClusterCount(footer)

	shortText := strings.Repeat("a", 200)
	if needsSummary(shortText, footer, footerLen) {
		t.Error("short text should not need summary")
	}

	longText := strings.Repeat("a", 300)
	if !needsSummary(longText, footer, footerLen) {
		t.Error("long text should need summary")
	}
}

func TestBuildReplyText(t *testing.T) {
	footer := "\n\n🌐 Translated by #DDaraBot"
	result := buildReplyText("Hello world", footer)
	want := "Hello world\n\n🌐 Translated by #DDaraBot"
	if result != want {
		t.Errorf("buildReplyText() = %q, want %q", result, want)
	}
}
