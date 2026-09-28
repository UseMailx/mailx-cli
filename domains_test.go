package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunDomainsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"dom_1","name":"example.com","ownership_state":"verified"}]}`))
	}))
	defer srv.Close()

	c, err := newClient(srv.URL, "k")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := runDomains(t.Context(), c, strings.NewReader(""), &buf, []string{"list"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"dom_1", "example.com", "verified"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunDomainsListEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runDomains(t.Context(), c, strings.NewReader(""), &buf, []string{"list"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No domains") {
		t.Fatalf("expected an empty-state message, got %q", buf.String())
	}
}

func TestRunDomainsInspect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains/dom_1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"dom_1","name":"example.com","ownership_state":"pending","records":[{"type":"TXT","name":"_mailx.example.com","value":"mailx-verify=abc"}]}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runDomains(t.Context(), c, strings.NewReader(""), &buf, []string{"inspect", "dom_1"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"example.com", "pending", "TXT", "_mailx.example.com", "mailx-verify=abc"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunDomainsUnknownVerb(t *testing.T) {
	c, _ := newClient("http://unused.invalid", "k")
	var buf bytes.Buffer
	if err := runDomains(t.Context(), c, strings.NewReader(""), &buf, []string{"purge", "dom_1"}); err == nil {
		t.Fatal("expected an error for an unsupported verb")
	}
}

func TestRunDomainsNoArgs(t *testing.T) {
	c, _ := newClient("http://unused.invalid", "k")
	var buf bytes.Buffer
	if err := runDomains(t.Context(), c, strings.NewReader(""), &buf, nil); err == nil {
		t.Fatal("expected a usage error")
	}
}

func TestRunDomainsCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/domains" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"dom_1","name":"example.com","ownership_state":"pending","records":[{"type":"TXT","name":"_mailx.example.com","value":"mailx-verify=abc"}]}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runDomains(t.Context(), c, strings.NewReader(""), &buf, []string{"create", "example.com"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"example.com", "dom_1", "pending", "TXT", "mailx-verify=abc"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunDomainsVerify(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/domains/dom_1/verify" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"dom_1","name":"example.com","ownership_state":"verified"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runDomains(t.Context(), c, strings.NewReader(""), &buf, []string{"verify", "dom_1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "verified") {
		t.Fatalf("expected verified in output, got %q", buf.String())
	}
}

func TestRunDomainsDeleteAsksForConfirmationAndAborts(t *testing.T) {
	// The server must never be called: an aborted confirmation must not
	// even attempt the DELETE.
	c, _ := newClient("http://unused.invalid", "k")
	var buf bytes.Buffer
	if err := runDomains(t.Context(), c, strings.NewReader("no\n"), &buf, []string{"delete", "dom_1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Aborted") {
		t.Fatalf("expected an abort message, got %q", buf.String())
	}
}

func TestRunDomainsDeleteConfirmed(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete || r.URL.Path != "/domains/dom_1" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runDomains(t.Context(), c, strings.NewReader("yes\n"), &buf, []string{"delete", "dom_1"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected the DELETE request to actually be sent after confirmation")
	}
	if !strings.Contains(buf.String(), "Deleted") {
		t.Fatalf("expected a deletion confirmation, got %q", buf.String())
	}
}

func TestRunDomainsDeleteYesFlagSkipsPrompt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	// An empty reader would normally abort (EOF), but --yes must skip the
	// prompt entirely - this is what makes the command scriptable.
	if err := runDomains(t.Context(), c, strings.NewReader(""), &buf, []string{"delete", "--yes", "dom_1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Deleted") {
		t.Fatalf("expected --yes to skip confirmation and delete, got %q", buf.String())
	}
}
