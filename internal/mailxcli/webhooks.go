package mailxcli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// webhook mirrors internal/api's public Webhook resource. Signature only
// ever appears in the create/rotate response (WebhookCreated), never here.
type webhook struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	CreatedAt string   `json:"created_at"`
}

type webhookCreated struct {
	webhook
	SigningSecret string `json:"signing_secret"`
}

type webhookList struct {
	Data []webhook `json:"data"`
}

type webhookDelivery struct {
	ID            string `json:"id"`
	EventID       string `json:"event_id"`
	Status        string `json:"status"`
	AttemptCount  int    `json:"attempt_count"`
	NextAttemptAt string `json:"next_attempt_at,omitempty"`
	CreatedAt     string `json:"created_at"`
}

type webhookDeliveryList struct {
	Data []webhookDelivery `json:"data"`
}

// runWebhooks dispatches `mailx-cli webhooks <verb>`. delete and
// rotate-secret are execute-tier: delete is irreversible and confirmed
// first unless --yes (spec section 25, same guard as domains/templates
// delete); rotate-secret invalidates the old signing secret immediately,
// so its new value is printed exactly once, same as create.
func runWebhooks(ctx context.Context, c *client, in io.Reader, out io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mailx-cli webhooks <list|get|create|delete|rotate-secret|deliveries> [args]")
	}
	switch args[0] {
	case "list":
		return runWebhooksList(ctx, c, out)
	case "get":
		if len(args) != 2 {
			return fmt.Errorf("usage: mailx-cli webhooks get WEBHOOK_ID")
		}
		return runWebhooksGet(ctx, c, out, args[1])
	case "create":
		fs := flag.NewFlagSet("webhooks create", flag.ContinueOnError)
		url := fs.String("url", "", "HTTPS endpoint to receive events (required)")
		events := fs.String("events", "", "comma-separated event types, e.g. email.delivered,email.bounced (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *url == "" || *events == "" {
			return fmt.Errorf("usage: mailx-cli webhooks create --url URL --events email.delivered,email.bounced,...")
		}
		return runWebhooksCreate(ctx, c, out, *url, strings.Split(*events, ","))
	case "delete":
		fs := flag.NewFlagSet("webhooks delete", flag.ContinueOnError)
		yes := fs.Bool("yes", false, "skip the confirmation prompt")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: mailx-cli webhooks delete [--yes] WEBHOOK_ID")
		}
		return runWebhooksDelete(ctx, c, in, out, fs.Arg(0), *yes)
	case "rotate-secret":
		if len(args) != 2 {
			return fmt.Errorf("usage: mailx-cli webhooks rotate-secret WEBHOOK_ID")
		}
		return runWebhooksRotateSecret(ctx, c, out, args[1])
	case "deliveries":
		if len(args) != 2 {
			return fmt.Errorf("usage: mailx-cli webhooks deliveries WEBHOOK_ID")
		}
		return runWebhooksDeliveries(ctx, c, out, args[1])
	default:
		return fmt.Errorf("unknown webhooks command %q (want list|get|create|delete|rotate-secret|deliveries)", args[0])
	}
}

func runWebhooksList(ctx context.Context, c *client, out io.Writer) error {
	var list webhookList
	if err := c.get(ctx, "/webhooks", &list); err != nil {
		return err
	}
	if len(list.Data) == 0 {
		fmt.Fprintln(out, "No webhooks.")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tURL\tEVENTS")
	for _, w := range list.Data {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", w.ID, w.URL, strings.Join(w.Events, ","))
	}
	return tw.Flush()
}

func runWebhooksGet(ctx context.Context, c *client, out io.Writer, id string) error {
	var w webhook
	if err := c.get(ctx, "/webhooks/"+id, &w); err != nil {
		return err
	}
	fmt.Fprintf(out, "Webhook\n  %s\n\n", w.ID)
	fmt.Fprintf(out, "URL\n  %s\n\n", w.URL)
	fmt.Fprintf(out, "Events\n  %s\n\n", strings.Join(w.Events, ", "))
	fmt.Fprintf(out, "Created\n  %s\n", w.CreatedAt)
	return nil
}

// runWebhooksCreate implements `mailx-cli webhooks create`: POST
// /webhooks, webhooks:write. The signing secret is returned exactly once -
// print it clearly and tell the caller to store it now.
func runWebhooksCreate(ctx context.Context, c *client, out io.Writer, url string, events []string) error {
	var w webhookCreated
	if err := c.post(ctx, "/webhooks", map[string]any{"url": url, "events": events}, &w); err != nil {
		return err
	}
	fmt.Fprintf(out, "Created webhook %s\n", w.ID)
	fmt.Fprintf(out, "URL: %s\n", w.URL)
	fmt.Fprintf(out, "Events: %s\n", strings.Join(w.Events, ", "))
	fmt.Fprintf(out, "\nSigning secret (store this now, MailX cannot show it again):\n  %s\n", w.SigningSecret)
	return nil
}

// runWebhooksDelete implements `mailx-cli webhooks delete`: DELETE
// /webhooks/{id}, webhooks:write, irreversible - same confirm-first
// pattern as domains/templates delete.
func runWebhooksDelete(ctx context.Context, c *client, in io.Reader, out io.Writer, id string, skipConfirm bool) error {
	ok, err := confirm(in, out, fmt.Sprintf("Delete webhook %s? This cannot be undone.", id), skipConfirm)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(out, "Aborted.")
		return nil
	}
	if err := c.deleteReq(ctx, "/webhooks/"+id); err != nil {
		return err
	}
	fmt.Fprintf(out, "Deleted webhook %s.\n", id)
	return nil
}

// runWebhooksRotateSecret implements `mailx-cli webhooks rotate-secret`:
// POST /webhooks/{id}/rotate-secret, webhooks:write. No confirmation
// prompt (matches API-key rotate/dkim create): it has a real effect
// (the old secret stops validating) but isn't destructive in the delete
// sense, and an operator rotating a leaked secret needs it to happen
// immediately, not behind a prompt.
func runWebhooksRotateSecret(ctx context.Context, c *client, out io.Writer, id string) error {
	var w webhookCreated
	if err := c.post(ctx, "/webhooks/"+id+"/rotate-secret", nil, &w); err != nil {
		return err
	}
	fmt.Fprintf(out, "Rotated signing secret for webhook %s\n", w.ID)
	fmt.Fprintf(out, "\nNew signing secret (store this now, MailX cannot show it again):\n  %s\n", w.SigningSecret)
	return nil
}

func runWebhooksDeliveries(ctx context.Context, c *client, out io.Writer, id string) error {
	var list webhookDeliveryList
	if err := c.get(ctx, "/webhooks/"+id+"/deliveries", &list); err != nil {
		return err
	}
	if len(list.Data) == 0 {
		fmt.Fprintln(out, "No deliveries.")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tATTEMPTS\tNEXT ATTEMPT\tCREATED")
	for _, d := range list.Data {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", d.ID, d.Status, d.AttemptCount, d.NextAttemptAt, d.CreatedAt)
	}
	return tw.Flush()
}
