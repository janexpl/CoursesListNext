package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// apiClient woła REST API CoursesList kluczem API. Serwer MCP nie ma własnej logiki ani
// dostępu do bazy: walidacja, uprawnienia (zakresy klucza) i audyt zostają po stronie API.
type apiClient struct {
	baseURL string // z /api/v1 na końcu
	apiKey  string
	http    *http.Client
}

func newAPIClient(siteURL, apiKey string) *apiClient {
	return &apiClient{
		baseURL: strings.TrimRight(siteURL, "/") + "/api/v1",
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("API zwróciło %d", e.Status)
	}
	return fmt.Sprintf("API zwróciło %d (%s): %s", e.Status, e.Code, e.Message)
}

func (c *apiClient) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, query, nil, false)
}

func (c *apiClient) send(ctx context.Context, method, path string, body any) ([]byte, error) {
	return c.do(ctx, method, path, nil, body, false)
}

// sendIdempotent wysyła Idempotency-Key wyliczony z trasy i treści. Agent, który ponowi
// to samo wywołanie (np. po zerwanym połączeniu), dostanie pierwszy wynik zamiast
// drugiego zaświadczenia.
func (c *apiClient) sendIdempotent(ctx context.Context, method, path string, body any) ([]byte, error) {
	return c.do(ctx, method, path, nil, body, true)
}

func (c *apiClient) do(ctx context.Context, method, path string, query url.Values, body any, idempotent bool) ([]byte, error) {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotent {
		sum := sha256.Sum256(append([]byte(method+" "+path+"\n"), payload...))
		req.Header.Set("Idempotency-Key", "mcp-"+hex.EncodeToString(sum[:16]))
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brak połączenia z API: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &envelope)
		return nil, &apiError{Status: resp.StatusCode, Code: envelope.Error.Code, Message: envelope.Error.Message}
	}
	if resp.StatusCode == http.StatusNoContent || len(raw) == 0 {
		return []byte(`{"ok":true}`), nil
	}
	return raw, nil
}
