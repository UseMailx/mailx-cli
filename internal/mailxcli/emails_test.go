package mailxcli

import (
	"bytes"
	"encoding/json"
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
	cases := [][]string{nil, {"get"}, {"diagnose"}, {"bogus"}, {"send"}, {"send", "--from", "a@example.com"}, {"send", "--to", "b@example.com"}}
	for _, args := range cases {
		if err := runEmails(t.Context(), c, &buf, args); err == nil {
			t.Fatalf("expected an error for args %v", args)
		}
	}
}

func TestRunEmailsSend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body sendEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.From != "a@example.com" || len(body.To) != 2 || body.To[0] != "b@example.com" || body.To[1] != "c@example.com" || body.Subject != "Hi" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"em_1","status":"queued"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	args := []string{"send", "--from", "a@example.com", "--to", "b@example.com", "--to", "c@example.com", "--subject", "Hi", "--text", "body"}
	if err := runEmails(t.Context(), c, &buf, args); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "em_1") || !strings.Contains(buf.String(), "queued") {
		t.Fatalf("expected the accepted message id/status in output, got %q", buf.String())
	}
}

func TestRunEmailsSendWithTemplateAndVariables(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body sendEmailRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.TemplateID != "tpl_1" || body.Variables["name"] != "Ada" {
			t.Fatalf("unexpected body: %+v", body)
		}
		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"em_2","status":"queued"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	args := []string{"send", "--from", "a@example.com", "--to", "b@example.com", "--template-id", "tpl_1", "--var", "name=Ada"}
	if err := runEmails(t.Context(), c, &buf, args); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "em_2") {
		t.Fatalf("expected em_2 in output, got %q", buf.String())
	}
}

func TestRunEmailsSendPassesIdempotencyKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Idempotency-Key"); got != "my-key-1" {
			t.Fatalf("expected Idempotency-Key header, got %q", got)
		}
		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"em_3","status":"queued"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	args := []string{"send", "--from", "a@example.com", "--to", "b@example.com", "--subject", "Hi", "--text", "body", "--idempotency-key", "my-key-1"}
	if err := runEmails(t.Context(), c, &buf, args); err != nil {
		t.Fatal(err)
	}
}

func TestRunEmailsSendInvalidVarRejected(t *testing.T) {
	c, _ := newClient("http://unused.invalid", "k")
	var buf bytes.Buffer
	args := []string{"send", "--from", "a@example.com", "--to", "b@example.com", "--template-id", "tpl_1", "--var", "not-a-kv-pair"}
	if err := runEmails(t.Context(), c, &buf, args); err == nil {
		t.Fatal("expected an error for a malformed --var")
	}
}
