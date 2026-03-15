package translator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/rivo/uniseg"
)

const maxGraphemes = 300

type Translator struct {
	g                   *genkit.Genkit
	model               string
	footer              string
	summarizeOnOverflow bool
	logger              *slog.Logger
}

func New(g *genkit.Genkit, model, footer string, summarizeOnOverflow bool, logger *slog.Logger) *Translator {
	return &Translator{
		g:                   g,
		model:               model,
		footer:              footer,
		summarizeOnOverflow: summarizeOnOverflow,
		logger:              logger,
	}
}

type TranslationResult struct {
	Lang string
	Text string
	Err  error
}

func (t *Translator) TranslateAll(ctx context.Context, text, sourceLang string, targetLangs []string) (map[string]string, []error) {
	ch := make(chan TranslationResult, len(targetLangs))
	for _, lang := range targetLangs {
		go func() {
			translated, err := t.translateWithRetry(ctx, text, sourceLang, lang)
			ch <- TranslationResult{Lang: lang, Text: translated, Err: err}
		}()
	}

	results := make(map[string]string)
	var errs []error
	for range targetLangs {
		r := <-ch
		if r.Err != nil {
			errs = append(errs, fmt.Errorf("translate to %s: %w", r.Lang, r.Err))
			t.logger.Error("translation failed", "lang", r.Lang, "error", r.Err)
		} else {
			results[r.Lang] = r.Text
		}
	}
	return results, errs
}

func (t *Translator) translateWithRetry(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		result, err := t.translate(ctx, text, sourceLang, targetLang)
		if err == nil {
			return result, nil
		}
		lastErr = err
		t.logger.Warn("translation attempt failed", "lang", targetLang, "attempt", attempt+1, "error", err)
	}
	return "", fmt.Errorf("after 3 retries: %w", lastErr)
}

func (t *Translator) translate(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	resp, err := genkit.Generate(ctx, t.g,
		ai.WithModelName(t.model),
		ai.WithSystem(buildTranslateSystemPrompt(sourceLang, targetLang)),
		ai.WithPrompt(text),
	)
	if err != nil {
		return "", err
	}

	translated := resp.Text()
	footerLen := uniseg.GraphemeClusterCount(t.footer)

	if t.summarizeOnOverflow && needsSummary(translated, t.footer, footerLen) {
		maxChars := maxGraphemes - footerLen
		resp, err = genkit.Generate(ctx, t.g,
			ai.WithModelName(t.model),
			ai.WithSystem(buildSummarizeSystemPrompt(sourceLang, targetLang, maxChars)),
			ai.WithPrompt(text),
		)
		if err != nil {
			return "", fmt.Errorf("summarize: %w", err)
		}
		translated = resp.Text()
	}

	return buildReplyText(translated, t.footer), nil
}

func needsSummary(text, footer string, footerLen int) bool {
	return uniseg.GraphemeClusterCount(text)+footerLen > maxGraphemes
}

func buildReplyText(translated, footer string) string {
	return translated + footer
}
