package mailxcli

import (
	"bytes"
	"encoding/json"
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
	if err := runTemplates(t.Context(), c, strings.NewReader(""), &buf, []string{"list"}); err != nil {
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
	if err := runTemplates(t.Context(), c, strings.NewReader(""), &buf, []string{"get", "tpl_1"}); err != nil {
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
	if err := runTemplates(t.Context(), c, strings.NewReader(""), &buf, []string{"preview", "tpl_1"}); err != nil {
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
	cases := [][]string{nil, {"get"}, {"preview"}, {"bogus"}, {"create"}, {"create", "--name", "x"}, {"update", "tpl_1"}}
	for _, args := range cases {
		if err := runTemplates(t.Context(), c, strings.NewReader(""), &buf, args); err == nil {
			t.Fatalf("expected an error for args %v", args)
		}
	}
}

func TestRunTemplatesCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/templates" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "welcome" || body["subject"] != "Welcome, {{name}}" {
			t.Fatalf("unexpected body: %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"tpl_1","name":"welcome","subject":"Welcome, {{name}}"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	args := []string{"create", "--name", "welcome", "--subject", "Welcome, {{name}}", "--text", "Hi {{name}}"}
	if err := runTemplates(t.Context(), c, strings.NewReader(""), &buf, args); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "tpl_1") {
		t.Fatalf("expected the new template id in output, got %q", buf.String())
	}
}

func TestRunTemplatesUpdateOnlySendsGivenFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/templates/tpl_1" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body) != 1 || body["subject"] != "New subject" {
			t.Fatalf("expected only subject in the PATCH body, got %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"tpl_1","name":"welcome","subject":"New subject"}`))
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")
	var buf bytes.Buffer
	if err := runTemplates(t.Context(), c, strings.NewReader(""), &buf, []string{"update", "--subject", "New subject", "tpl_1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Updated template") {
		t.Fatalf("expected an update confirmation, got %q", buf.String())
	}
}

func TestRunTemplatesDeleteConfirmedAndYesFlag(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete || r.URL.Path != "/templates/tpl_1" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c, _ := newClient(srv.URL, "k")

	var buf bytes.Buffer
	if err := runTemplates(t.Context(), c, strings.NewReader("no\n"), &buf, []string{"delete", "tpl_1"}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("a declined confirmation must never call the server")
	}

	buf.Reset()
	if err := runTemplates(t.Context(), c, strings.NewReader(""), &buf, []string{"delete", "--yes", "tpl_1"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected --yes to skip the prompt and actually delete")
	}
}
