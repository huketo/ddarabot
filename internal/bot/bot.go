package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/huketo/ddarabot/internal/bluesky"
	"github.com/huketo/ddarabot/internal/config"
	"github.com/huketo/ddarabot/internal/filter"
	"github.com/huketo/ddarabot/internal/jetstream"
	"github.com/huketo/ddarabot/internal/store"
	"github.com/huketo/ddarabot/internal/translator"
)

type Bot struct {
	cfg        *config.Config
	store      *store.Store
	translator *translator.Translator
	poster     *bluesky.Poster
	listener   *jetstream.Listener
	eventCh    chan jetstream.Post
	logger     *slog.Logger
}

func New(
	cfg *config.Config,
	did string,
	st *store.Store,
	tr *translator.Translator,
	poster *bluesky.Poster,
	logger *slog.Logger,
) *Bot {
	eventCh := make(chan jetstream.Post, 64)
	listener := jetstream.NewListener(
		cfg.Jetstream.URL,
		did,
		logger,
		eventCh,
		st.SaveCursor,
	)

	return &Bot{
		cfg:        cfg,
		store:      st,
		translator: tr,
		poster:     poster,
		listener:   listener,
		eventCh:    eventCh,
		logger:     logger,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	var cursor *int64
	if c, ok := b.store.GetCursor(); ok {
		cursor = &c
		b.logger.Info("resuming from cursor", "cursor", c)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- b.listener.Run(ctx, cursor)
	}()

	for {
		select {
		case post := <-b.eventCh:
			b.processPost(ctx, post)
		case err := <-errCh:
			return fmt.Errorf("jetstream listener: %w", err)
		case <-ctx.Done():
			b.logger.Info("shutting down bot")
			return ctx.Err()
		}
	}
}

func (b *Bot) processPost(ctx context.Context, post jetstream.Post) {
	triggerTag := b.cfg.Translation.TriggerHashtag

	if !filter.HasTriggerTag(post.Facets, triggerTag) {
		return
	}

	if filter.IsReply(post.Record) {
		b.logger.Debug("skipping reply", "did", post.DID, "rkey", post.RKey)
		return
	}

	uri := buildPostURI(post.DID, post.RKey)

	// Determine which languages still need processing
	doneLangs := b.store.ProcessedLanguages(uri)
	var remaining []string
	for _, lang := range b.cfg.Translation.TargetLanguages {
		if !doneLangs[lang] {
			remaining = append(remaining, lang)
		}
	}
	if len(remaining) == 0 {
		b.logger.Debug("already processed", "uri", uri)
		return
	}

	cleanText := filter.RemoveTriggerTag(post.Text, post.Facets, triggerTag)
	b.logger.Info("processing post", "uri", uri, "text", cleanText, "remaining", remaining)

	translations, errs := b.translator.TranslateAll(
		ctx,
		cleanText,
		b.cfg.Translation.SourceLanguage,
		remaining,
	)
	for _, err := range errs {
		b.logger.Error("translation error", "error", err)
	}

	if len(translations) == 0 {
		b.logger.Error("all translations failed, skipping post", "uri", uri)
		return
	}

	original := bluesky.OriginalPost{URI: uri, CID: post.CID}
	postErrs := b.poster.PostAll(ctx, original, translations, 3)

	// Collect only successfully posted languages
	failed := make(map[string]bool)
	for _, err := range postErrs {
		b.logger.Error("posting error", "error", err)
		for lang := range translations {
			if contains(err.Error(), lang) {
				failed[lang] = true
			}
		}
	}

	var succeeded []string
	for lang := range translations {
		if !failed[lang] {
			succeeded = append(succeeded, lang)
		}
	}

	if len(succeeded) > 0 {
		// Merge with previously completed languages
		allDone := make([]string, 0, len(doneLangs)+len(succeeded))
		for lang := range doneLangs {
			allDone = append(allDone, lang)
		}
		allDone = append(allDone, succeeded...)
		if err := b.store.MarkProcessed(uri, allDone); err != nil {
			b.logger.Error("mark processed failed", "uri", uri, "error", err)
		}
	}
}

func buildPostURI(did, rkey string) string {
	return "at://" + did + "/app.bsky.feed.post/" + rkey
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
}
