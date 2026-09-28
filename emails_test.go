package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunEmailsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"em_1","status":"delivered","subject":"Hi","to":["a@example.com","b@example.com"]}]}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runEmails(t.Context(), c, &buf, []string{"list"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"em_1", "delivered", "Hi", "a@example.com", "+1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunEmailsGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails/em_1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"em_1","from":"a@example.com","to":["b@example.com"],"subject":"Hi","status":"queued"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runEmails(t.Context(), c, &buf, []string{"get", "em_1"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"em_1", "a@example.com", "b@example.com", "Hi", "queued"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunEmailsDiagnose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails/em_1/events" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message_id": "em_1",
			"status": "failed",
			"events": [{"type": "email.bounced", "occurred_at": "2026-09-28T09:00:00Z"}],
			"attempts": [{"attempt_number": 1, "decision": "terminal_failure", "recipient": "b@example.com", "accepted": false, "final_code": 550, "enhanced_status": "5.1.1", "remote_message": "user unknown", "failure_stage": "rcpt_to"}]
		}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runEmails(t.Context(), c, &buf, []string{"diagnose", "em_1"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"em_1", "failed", "email.bounced", "b@example.com", "terminal_failure", "550 5.1.1", "user unknown", "rcpt_to"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunEmailsUsageErrors(t *testing.T) {
	c, _ := newClient("http://unused.invalid", "k")
	var buf bytes.Buffer
	cases := [][]string{nil, {"get"}, {"diagnose"}, {"bogus"}}
	for _, args := range cases {
		if err := runEmails(t.Context(), c, &buf, args); err == nil {
			t.Fatalf("expected an error for args %v", args)
		}
	}
}
