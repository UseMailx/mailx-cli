package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunTemplatesList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/templates" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"tpl_1","name":"welcome","subject":"Welcome, {{name}}"}]}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runTemplates(t.Context(), c, &buf, []string{"list"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"tpl_1", "welcome", "Welcome, {{name}}"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunTemplatesGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/templates/tpl_1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"tpl_1","name":"welcome","subject":"Welcome, {{name}}","text":"Hi {{name}}","html":"<h1>Hi {{name}}</h1>"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runTemplates(t.Context(), c, &buf, []string{"get", "tpl_1"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"welcome", "tpl_1", "Welcome, {{name}}", "Hi {{name}}", "<h1>Hi {{name}}</h1>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunTemplatesPreview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/templates/tpl_1/preview" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"subject":"Welcome, ","text":"Hi ","html":"<h1>Hi </h1>"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runTemplates(t.Context(), c, &buf, []string{"preview", "tpl_1"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"Welcome,", "Hi ", "<h1>Hi </h1>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunTemplatesUsageErrors(t *testing.T) {
	c, _ := newClient("http://unused.invalid", "k")
	var buf bytes.Buffer
	cases := [][]string{nil, {"get"}, {"preview"}, {"bogus"}}
	for _, args := range cases {
		if err := runTemplates(t.Context(), c, &buf, args); err == nil {
			t.Fatalf("expected an error for args %v", args)
		}
	}
}
