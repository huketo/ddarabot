package bluesky

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

type Auth struct {
	pdsHost        string
	handle         string
	password       string
	client         *http.Client
	session        *Session
	lastRefreshJwt string
	mu             sync.RWMutex
}

func NewAuth(pdsHost, handle, password string) *Auth {
	return &Auth{
		pdsHost:  pdsHost,
		handle:   handle,
		password: password,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *Auth) GetSession(ctx context.Context) (*Session, error) {
	a.mu.RLock()
	s := a.session
	a.mu.RUnlock()
	if s != nil {
		return s, nil
	}

	// Try refreshing first if we have a refresh token
	if refreshed, err := a.RefreshSession(ctx); err == nil {
		return refreshed, nil
	}

	return a.CreateSession(ctx)
}

func (a *Auth) CreateSession(ctx context.Context) (*Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check: another goroutine may have restored the session
	if a.session != nil {
		return a.session, nil
	}

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
	a.lastRefreshJwt = session.RefreshJwt
	return &session, nil
}

func (a *Auth) RefreshSession(ctx context.Context) (*Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check: another goroutine may have restored the session
	if a.session != nil {
		return a.session, nil
	}

	refreshJwt := a.lastRefreshJwt
	if refreshJwt == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.pdsHost+"/xrpc/com.atproto.server.refreshSession", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+refreshJwt)

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
	a.lastRefreshJwt = session.RefreshJwt
	return &session, nil
}

func (a *Auth) InvalidateSession() {
	a.mu.Lock()
	a.session = nil
	a.mu.Unlock()
}

type resolveHandleResponse struct {
	DID string `json:"did"`
}

// ResolveDID resolves a Bluesky handle to a DID using the public XRPC endpoint.
// This does not require authentication.
func ResolveDID(ctx context.Context, pdsHost, handle string, client ...*http.Client) (string, error) {
	u, err := url.Parse(pdsHost + "/xrpc/com.atproto.identity.resolveHandle")
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	q := u.Query()
	q.Set("handle", handle)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}

	httpClient := defaultHTTPClient
	if len(client) > 0 && client[0] != nil {
		httpClient = client[0]
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve handle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var xrpcErr XRPCError
		json.NewDecoder(resp.Body).Decode(&xrpcErr)
		return "", fmt.Errorf("resolve handle: %s %s", xrpcErr.Error, xrpcErr.Message)
	}

	var result resolveHandleResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode resolve handle: %w", err)
	}
	return result.DID, nil
}
