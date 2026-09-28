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
	if err := runDomains(t.Context(), c, &buf, []string{"list"}); err != nil {
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
	if err := runDomains(t.Context(), c, &buf, []string{"list"}); err != nil {
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
	if err := runDomains(t.Context(), c, &buf, []string{"inspect", "dom_1"}); err != nil {
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
	if err := runDomains(t.Context(), c, &buf, []string{"delete", "dom_1"}); err == nil {
		t.Fatal("expected an error for an unsupported verb")
	}
}

func TestRunDomainsNoArgs(t *testing.T) {
	c, _ := newClient("http://unused.invalid", "k")
	var buf bytes.Buffer
	if err := runDomains(t.Context(), c, &buf, nil); err == nil {
		t.Fatal("expected a usage error")
	}
}
