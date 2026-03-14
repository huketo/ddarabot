package jetstream

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	jsclient "github.com/bluesky-social/jetstream/pkg/client"
	jsmodels "github.com/bluesky-social/jetstream/pkg/models"
)

// Post represents a parsed Bluesky post event from the Jetstream firehose.
type Post struct {
	DID    string
	RKey   string
	CID    string
	TimeUS int64
	Text   string
	Facets json.RawMessage
	Record json.RawMessage
}

type postRecord struct {
	Type   string           `json:"$type"`
	Text   string           `json:"text"`
	Facets json.RawMessage  `json:"facets"`
	Reply  *json.RawMessage `json:"reply"`
}

// Listener connects to a Bluesky Jetstream endpoint and emits Post events.
type Listener struct {
	config     *jsclient.ClientConfig
	logger     *slog.Logger
	eventCh    chan<- Post
	saveCursor func(int64) error
}

// NewListener creates a Listener configured for the given Jetstream WebSocket
// URL and set of watched DIDs.
func NewListener(
	url string,
	did string,
	logger *slog.Logger,
	eventCh chan<- Post,
	saveCursor func(int64) error,
) *Listener {
	cfg := &jsclient.ClientConfig{
		Compress:          true,
		WebsocketURL:      url,
		WantedDids:        []string{did},
		WantedCollections: []string{"app.bsky.feed.post"},
		ExtraHeaders:      map[string]string{},
	}
	return &Listener{
		config:     cfg,
		logger:     logger,
		eventCh:    eventCh,
		saveCursor: saveCursor,
	}
}

// scheduler implements jsclient.Scheduler so the Jetstream client can
// dispatch decoded events to the listener's channel.
type scheduler struct {
	listener *Listener
	count    int64
	lastSave time.Time
}

func (s *scheduler) AddWork(ctx context.Context, repo string, evt *jsmodels.Event) error {
	if evt.Kind != jsmodels.EventKindCommit || evt.Commit == nil {
		return nil
	}
	if evt.Commit.Operation != jsmodels.CommitOperationCreate {
		return nil
	}

	var rec postRecord
	if err := json.Unmarshal(evt.Commit.Record, &rec); err != nil {
		return nil // skip unparseable records
	}

	post := Post{
		DID:    evt.Did,
		RKey:   evt.Commit.RKey,
		CID:    evt.Commit.CID,
		TimeUS: evt.TimeUS,
		Text:   rec.Text,
		Facets: rec.Facets,
		Record: evt.Commit.Record,
	}

	select {
	case s.listener.eventCh <- post:
	case <-ctx.Done():
		return ctx.Err()
	}

	s.count++
	if s.count%100 == 0 || time.Since(s.lastSave) >= 30*time.Second {
		if err := s.listener.saveCursor(evt.TimeUS); err != nil {
			s.listener.logger.Warn("failed to save cursor", "error", err)
		}
		s.lastSave = time.Now()
	}

	return nil
}

func (s *scheduler) Shutdown() {}

// Run connects to the Jetstream WebSocket and processes events until the
// context is cancelled. It reconnects automatically with exponential backoff.
func (l *Listener) Run(ctx context.Context, cursor *int64) error {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		sched := &scheduler{listener: l, lastSave: time.Now()}
		client, err := jsclient.NewClient(l.config, l.logger, sched)
		if err != nil {
			return fmt.Errorf("create jetstream client: %w", err)
		}

		l.logger.Info("connecting to jetstream", "url", l.config.WebsocketURL)
		err = client.ConnectAndRead(ctx, cursor)

		if ctx.Err() != nil {
			if cursor != nil {
				_ = l.saveCursor(*cursor)
			}
			return ctx.Err()
		}

		l.logger.Warn("jetstream disconnected, reconnecting", "error", err, "backoff", backoff)

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
