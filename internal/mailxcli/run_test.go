package mailxcli

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunNoArgsPrintsUsage(t *testing.T) {
	if code := Run(nil); code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	if code := Run([]string{"send-a-million-emails"}); code != 2 {
		t.Fatalf("expected exit code 2 for an unknown command, got %d", code)
	}
}

func TestRunHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		if code := Run(args); code != 0 {
			t.Fatalf("%v: expected exit code 0, got %d", args, code)
		}
	}
}

func TestRunMissingAPIKey(t *testing.T) {
	t.Setenv("MAILX_API_KEY", "")
	if code := Run([]string{"whoami"}); code != 1 {
		t.Fatalf("expected exit code 1 when MAILX_API_KEY is unset, got %d", code)
	}
}

func TestRunWhoamiEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"organization":{"id":"tn_1","name":"Acme"},"api_key":{"id":"mx_1","name":"k","scopes":[]}}`))
	}))
	defer srv.Close()

	t.Setenv("MAILX_API_KEY", "test-key")
	t.Setenv("MAILX_API_BASE_URL", srv.URL)
	if code := Run([]string{"whoami"}); code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}
