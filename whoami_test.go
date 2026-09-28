package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunWhoamiPrintsOrgCredentialAndScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/whoami" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("expected Authorization: Bearer test-key, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"organization": {"id": "tn_1", "name": "Acme"},
			"api_key": {"id": "mx_7f2a", "name": "Production", "scopes": ["emails:send", "domains:read"]}
		}`))
	}))
	defer srv.Close()

	c, err := newClient(srv.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := runWhoami(t.Context(), c, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"Acme", "mx_7f2a", "Production", "emails:send", "domains:read"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunWhoamiSurfacesServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","code":"invalid_api_key","message":"invalid API key"}}`))
	}))
	defer srv.Close()

	c, err := newClient(srv.URL, "bad-key")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = runWhoami(t.Context(), c, &buf)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "invalid API key") {
		t.Fatalf("expected the server's own message in the error, got %q", err)
	}
}

func TestNewClientRequiresAPIKey(t *testing.T) {
	if _, err := newClient("", ""); err == nil {
		t.Fatal("expected an error when MAILX_API_KEY is unset")
	}
}
