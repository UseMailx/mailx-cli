// Command mailx-cli is MailX's developer-facing CLI: a thin client over the
// Product API, authenticated the same way any SDK or the MCP server is
// (an API key, via MAILX_API_KEY/MAILX_API_BASE_URL - the same override
// convention DEC-200 established for every other client). It is NOT the
// operator/admin tool (see cmd/mailx, which talks directly to Postgres) -
// this binary never touches the database, a queue, or any internal
// package; every command is an HTTP call to a documented /v1 route.
//
// This is intentionally a small foundation, not the full CLI the platform
// spec describes: one command (whoami) proving the dispatch shape, ready
// for `emails`, `domains`, `templates` etc. to be added as their own files
// following the same pattern - each a thin wrapper over an existing API
// capability, never new business logic living only in the CLI.
package main

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
	{name: "domains", summary: "list | inspect DOMAIN_ID", run: func(ctx context.Context, c *client, args []string) error {
		return runDomains(ctx, c, os.Stdout, args)
	}},
	{name: "emails", summary: "list | get MESSAGE_ID | diagnose MESSAGE_ID", run: func(ctx context.Context, c *client, args []string) error {
		return runEmails(ctx, c, os.Stdout, args)
	}},
	{name: "templates", summary: "list | get TEMPLATE_ID | preview TEMPLATE_ID", run: func(ctx context.Context, c *client, args []string) error {
		return runTemplates(ctx, c, os.Stdout, args)
	}},
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
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
