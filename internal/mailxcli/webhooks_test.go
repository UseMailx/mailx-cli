package mailxcli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunWebhooksList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/webhooks" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"wh_1","url":"https://example.com/hook","events":["email.delivered","email.bounced"],"created_at":"2026-01-01T00:00:00Z"}]}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runWebhooks(t.Context(), c, strings.NewReader(""), &buf, []string{"list"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"wh_1", "https://example.com/hook", "email.delivered", "email.bounced"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunWebhooksListEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runWebhooks(t.Context(), c, strings.NewReader(""), &buf, []string{"list"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No webhooks") {
		t.Fatalf("expected an empty-state message, got %q", buf.String())
	}
}

func TestRunWebhooksGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/webhooks/wh_1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"wh_1","url":"https://example.com/hook","events":["email.delivered"],"created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runWebhooks(t.Context(), c, strings.NewReader(""), &buf, []string{"get", "wh_1"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"wh_1", "https://example.com/hook", "email.delivered", "2026-01-01"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunWebhooksCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/webhooks" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["url"] != "https://example.com/hook" {
			t.Fatalf("unexpected url in body: %+v", body)
		}
		events, _ := body["events"].([]any)
		if len(events) != 2 || events[0] != "email.delivered" || events[1] != "email.bounced" {
			t.Fatalf("unexpected events in body: %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"wh_1","url":"https://example.com/hook","events":["email.delivered","email.bounced"],"created_at":"2026-01-01T00:00:00Z","signing_secret":"whsec_abc123"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	args := []string{"create", "--url", "https://example.com/hook", "--events", "email.delivered,email.bounced"}
	if err := runWebhooks(t.Context(), c, strings.NewReader(""), &buf, args); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"wh_1", "whsec_abc123"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunWebhooksCreateMissingFlagsRejected(t *testing.T) {
	c, _ := newClient("http://unused.invalid", "k")
	var buf bytes.Buffer
	cases := [][]string{
		{"create"},
		{"create", "--url", "https://example.com/hook"},
		{"create", "--events", "email.delivered"},
	}
	for _, args := range cases {
		if err := runWebhooks(t.Context(), c, strings.NewReader(""), &buf, args); err == nil {
			t.Fatalf("expected an error for args %v", args)
		}
	}
}

func TestRunWebhooksRotateSecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/webhooks/wh_1/rotate-secret" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"wh_1","url":"https://example.com/hook","events":["email.delivered"],"created_at":"2026-01-01T00:00:00Z","signing_secret":"whsec_new"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runWebhooks(t.Context(), c, strings.NewReader(""), &buf, []string{"rotate-secret", "wh_1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "whsec_new") {
		t.Fatalf("expected the new secret in output, got %q", buf.String())
	}
}

func TestRunWebhooksDeliveries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/webhooks/wh_1/deliveries" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"whd_1","event_id":"ev_1","status":"succeeded","attempt_count":1,"created_at":"2026-01-01T00:00:00Z"}]}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runWebhooks(t.Context(), c, strings.NewReader(""), &buf, []string{"deliveries", "wh_1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "whd_1") || !strings.Contains(buf.String(), "succeeded") {
		t.Fatalf("expected delivery row in output, got %q", buf.String())
	}
}

func TestRunWebhooksDeleteConfirmedAndYesFlag(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete || r.URL.Path != "/webhooks/wh_1" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")

	var buf bytes.Buffer
	if err := runWebhooks(t.Context(), c, strings.NewReader("no\n"), &buf, []string{"delete", "wh_1"}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("a declined confirmation must never call the server")
	}

	buf.Reset()
	if err := runWebhooks(t.Context(), c, strings.NewReader(""), &buf, []string{"delete", "--yes", "wh_1"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected --yes to skip the prompt and actually delete")
	}
}

func TestRunWebhooksUsageErrors(t *testing.T) {
	c, _ := newClient("http://unused.invalid", "k")
	var buf bytes.Buffer
	cases := [][]string{nil, {"get"}, {"rotate-secret"}, {"deliveries"}, {"bogus"}}
	for _, args := range cases {
		if err := runWebhooks(t.Context(), c, strings.NewReader(""), &buf, args); err == nil {
			t.Fatalf("expected an error for args %v", args)
		}
	}
}
