package bot

import (
	"context"
	"fmt"
	"log/slog"

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
	if b.store.IsProcessed(uri) {
		b.logger.Debug("already processed", "uri", uri)
		return
	}

	cleanText := filter.RemoveTriggerTag(post.Text, post.Facets, triggerTag)
	b.logger.Info("processing post", "uri", uri, "text", cleanText)

	translations, errs := b.translator.TranslateAll(
		ctx,
		cleanText,
		b.cfg.Translation.SourceLanguage,
		b.cfg.Translation.TargetLanguages,
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
	for _, err := range postErrs {
		b.logger.Error("posting error", "error", err)
	}

	langs := make([]string, 0, len(translations))
	for lang := range translations {
		langs = append(langs, lang)
	}
	if err := b.store.MarkProcessed(uri, langs); err != nil {
		b.logger.Error("mark processed failed", "uri", uri, "error", err)
	}
}

func buildPostURI(did, rkey string) string {
	return "at://" + did + "/app.bsky.feed.post/" + rkey
}
