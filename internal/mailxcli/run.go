// Package mailxcli is MailX's developer-facing CLI: a thin client over the
// Product API, authenticated the same way any SDK or the MCP server is
// (an API key, via MAILX_API_KEY/MAILX_API_BASE_URL - the same override
// convention every other MailX client uses). It contains no business logic
// of its own; every command is an HTTP call to a documented /v1 route.
//
// The actual binary entrypoint is cmd/mailx-cli/main.go, which does nothing
// but call Run - keeping this package importable/testable on its own and
// leaving room for other entrypoints (e.g. a future plugin host) later.
package mailxcli

import (
	"context"
	"fmt"
	"os"
)

// command is one CLI subcommand: a name, a one-line summary for `help`,
// and a run function taking the already-parsed remaining args.
type command struct {
	name    string
	summary string
	run     func(ctx context.Context, c *client, args []string) error
}

// commands is the whole dispatch table. Adding a subcommand later means
// adding one entry here and one new file - never touching this list's
// existing entries or the dispatch logic below.
var commands = []command{
	{name: "whoami", summary: "Show the authenticated organization, credential, and scopes", run: func(ctx context.Context, c *client, _ []string) error {
		return runWhoami(ctx, c, os.Stdout)
	}},
	{name: "domains", summary: "list | inspect | create | verify | delete | dkim | spf | dmarc | bimi", run: func(ctx context.Context, c *client, args []string) error {
		return runDomains(ctx, c, os.Stdin, os.Stdout, args)
	}},
	{name: "emails", summary: "list | get | diagnose | send", run: func(ctx context.Context, c *client, args []string) error {
		return runEmails(ctx, c, os.Stdout, args)
	}},
	{name: "templates", summary: "list | get | preview | create | update | delete", run: func(ctx context.Context, c *client, args []string) error {
		return runTemplates(ctx, c, os.Stdin, os.Stdout, args)
	}},
	{name: "webhooks", summary: "list | get | create | delete | rotate-secret | deliveries", run: func(ctx context.Context, c *client, args []string) error {
		return runWebhooks(ctx, c, os.Stdin, os.Stdout, args)
	}},
}

// Run executes one CLI invocation and returns the process exit code -
// cmd/mailx-cli/main.go's only job is os.Exit(mailxcli.Run(os.Args[1:])).
func Run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 2
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printUsage()
		return 0
	}
	for _, cmd := range commands {
		if cmd.name != args[0] {
			continue
		}
		c, err := newClient(os.Getenv("MAILX_API_BASE_URL"), os.Getenv("MAILX_API_KEY"))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		if err := cmd.run(context.Background(), c, args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "mailx-cli: unknown command %q\n\n", args[0])
	printUsage()
	return 2
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: mailx-cli <command>")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	for _, cmd := range commands {
		fmt.Fprintf(os.Stderr, "  %-10s %s\n", cmd.name, cmd.summary)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Configuration (environment variables):")
	fmt.Fprintln(os.Stderr, "  MAILX_API_KEY       required")
	fmt.Fprintln(os.Stderr, "  MAILX_API_BASE_URL  optional, defaults to", defaultBaseURL)
}
