package jetstream

import (
	"log/slog"
	"testing"
)

func TestNewListener(t *testing.T) {
	ch := make(chan Post, 10)
	l := NewListener(
		"wss://jetstream2.us-east.bsky.network/subscribe",
		[]string{"did:plc:test"},
		slog.Default(),
		ch,
		func(cursor int64) error { return nil },
	)

	if l.config.WebsocketURL != "wss://jetstream2.us-east.bsky.network/subscribe" {
		t.Errorf("URL = %q", l.config.WebsocketURL)
	}
	if len(l.config.WantedDids) != 1 || l.config.WantedDids[0] != "did:plc:test" {
		t.Errorf("WantedDids = %v", l.config.WantedDids)
	}
	if !l.config.Compress {
		t.Error("Compress should be true")
	}
}
