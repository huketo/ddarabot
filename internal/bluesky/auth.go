package bluesky

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Auth struct {
	pdsHost  string
	handle   string
	password string
	client   *http.Client
	session  *Session
	mu       sync.RWMutex
}

func NewAuth(pdsHost, handle, password string) *Auth {
	return &Auth{
		pdsHost:  pdsHost,
		handle:   handle,
		password: password,
		client:   &http.Client{},
	}
}

func (a *Auth) GetSession(ctx context.Context) (*Session, error) {
	a.mu.RLock()
	s := a.session
	a.mu.RUnlock()
	if s != nil {
		return s, nil
	}
	return a.CreateSession(ctx)
}

func (a *Auth) CreateSession(ctx context.Context) (*Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	body, err := json.Marshal(CreateSessionRequest{
		Identifier: a.handle,
		Password:   a.password,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.pdsHost+"/xrpc/com.atproto.server.createSession",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var xrpcErr XRPCError
		json.NewDecoder(resp.Body).Decode(&xrpcErr)
		return nil, fmt.Errorf("create session: %s %s", xrpcErr.Error, xrpcErr.Message)
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}

	a.session = &session
	return &session, nil
}

func (a *Auth) RefreshSession(ctx context.Context) (*Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session == nil {
		return nil, fmt.Errorf("no session to refresh")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.pdsHost+"/xrpc/com.atproto.server.refreshSession", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.session.RefreshJwt)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.session = nil
		return nil, fmt.Errorf("refresh session: status %d", resp.StatusCode)
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}

	a.session = &session
	return &session, nil
}

func (a *Auth) InvalidateSession() {
	a.mu.Lock()
	a.session = nil
	a.mu.Unlock()
}
