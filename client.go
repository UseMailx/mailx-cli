package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// client is a thin wrapper over the MailX Product API. It authenticates
// with an API key exactly like every other client (SDKs, MCP) - the CLI is
// a client over the API, not a second backend (spec section 2), so it
// contains no business logic of its own, only request/response plumbing.
type client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// defaultBaseURL matches the SDKs'/MCP's own default (DEC-200): overridable
// per environment, never hardcoded into a self-hosted deployment's path.
const defaultBaseURL = "https://api.mailx.dev/v1"

func newClient(baseURL, apiKey string) (*client, error) {
	if apiKey == "" {
		return nil, errMissingAPIKey
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &client{baseURL: strings.TrimSuffix(baseURL, "/"), apiKey: apiKey, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

var errMissingAPIKey = fmt.Errorf("MAILX_API_KEY is not set")

// apiError mirrors the server's APIError envelope closely enough to surface
// a useful message - it never needs the full schema, since the CLI only
// ever displays it, never branches on the error code.
type apiError struct {
	Error struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// get performs an authenticated GET and decodes a 2xx JSON body into out.
// A non-2xx response is returned as an error carrying the server's own
// message, never a generic "request failed" - CLI output should be at
// least as informative as the API's own structured error model (spec
// section 28).
func (c *client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var ae apiError
		if json.Unmarshal(body, &ae) == nil && ae.Error.Message != "" {
			return fmt.Errorf("%s (%s)", ae.Error.Message, ae.Error.Code)
		}
		return fmt.Errorf("unexpected response: %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}
