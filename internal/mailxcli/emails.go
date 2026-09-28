package mailxcli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// email/emailList mirror internal/api's public Email resource.
type email struct {
	ID      string   `json:"id"`
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Status  string   `json:"status"`
}

type emailList struct {
	Data []email `json:"data"`
}

// emailDiagnostics mirrors GET /emails/{id}/events' response shape
// (internal/api's emailDiagnostics) - deliberately duplicated here rather
// than imported, same as every other resource type in this package.
type emailDiagnostics struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
	Events    []struct {
		Type       string `json:"type"`
		OccurredAt string `json:"occurred_at"`
	} `json:"events"`
	Attempts []struct {
		AttemptNumber  int    `json:"attempt_number"`
		Decision       string `json:"decision"`
		Recipient      string `json:"recipient"`
		Accepted       bool   `json:"accepted"`
		FinalCode      int    `json:"final_code"`
		EnhancedStatus string `json:"enhanced_status"`
		RemoteMessage  string `json:"remote_message"`
		FailureStage   string `json:"failure_stage"`
	} `json:"attempts"`
}

// runEmails dispatches `mailx-cli emails <verb>`. send is execute-tier
// (spec section 11) - it goes through the exact same POST /emails
// sending-safety path (suppression, idempotency, plan limits) as every
// other client; the CLI adds no send logic of its own.
func runEmails(ctx context.Context, c *client, out io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mailx-cli emails <list|get|diagnose|send> [args]")
	}
	switch args[0] {
	case "list":
		return runEmailsList(ctx, c, out)
	case "get":
		if len(args) != 2 {
			return fmt.Errorf("usage: mailx-cli emails get MESSAGE_ID")
		}
		return runEmailsGet(ctx, c, out, args[1])
	case "diagnose":
		if len(args) != 2 {
			return fmt.Errorf("usage: mailx-cli emails diagnose MESSAGE_ID")
		}
		return runEmailsDiagnose(ctx, c, out, args[1])
	case "send":
		return runEmailsSendCmd(ctx, c, out, args[1:])
	default:
		return fmt.Errorf("unknown emails command %q (want list|get|diagnose|send)", args[0])
	}
}

// repeatableFlag collects every occurrence of a flag.Value flag (e.g.
// --to a@x.com --to b@x.com) instead of the stdlib's default of only
// keeping the last one.
type repeatableFlag []string

func (r *repeatableFlag) String() string { return strings.Join(*r, ",") }
func (r *repeatableFlag) Set(v string) error {
	*r = append(*r, v)
	return nil
}

// runEmailsSendCmd parses `mailx-cli emails send`'s flags and calls
// runEmailsSend. Kept separate from the flag-parsing-free verbs above so
// their signatures stay simple.
func runEmailsSendCmd(ctx context.Context, c *client, out io.Writer, args []string) error {
	fs := flag.NewFlagSet("emails send", flag.ContinueOnError)
	from := fs.String("from", "", "sender address (required)")
	var to, cc, bcc, vars repeatableFlag
	fs.Var(&to, "to", "recipient address (repeatable)")
	fs.Var(&cc, "cc", "cc address (repeatable)")
	fs.Var(&bcc, "bcc", "bcc address (repeatable)")
	subject := fs.String("subject", "", "subject (ignored if --template-id is given)")
	text := fs.String("text", "", "plain-text body (ignored if --template-id is given)")
	html := fs.String("html", "", "HTML body (ignored if --template-id is given)")
	templateID := fs.String("template-id", "", "send a Template instead of raw subject/text/html")
	fs.Var(&vars, "var", "template variable as key=value (repeatable, only with --template-id)")
	idempotencyKey := fs.String("idempotency-key", "", "optional Idempotency-Key: a retry with the same key and request never double-sends")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *from == "" {
		return fmt.Errorf("usage: mailx-cli emails send --from ADDR --to ADDR [...] (--subject S --text T | --template-id ID)")
	}
	if len(to) == 0 {
		return fmt.Errorf("emails send: at least one --to is required")
	}
	variables := map[string]string{}
	for _, kv := range vars {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return fmt.Errorf("emails send: --var must be key=value, got %q", kv)
		}
		variables[k] = v
	}
	return runEmailsSend(ctx, c, out, sendEmailRequest{
		From: *from, To: to, Cc: cc, Bcc: bcc, Subject: *subject, Text: *text, HTML: *html,
		TemplateID: *templateID, Variables: variables,
	}, *idempotencyKey)
}

func runEmailsList(ctx context.Context, c *client, out io.Writer) error {
	var list emailList
	if err := c.get(ctx, "/emails", &list); err != nil {
		return err
	}
	if len(list.Data) == 0 {
		fmt.Fprintln(out, "No emails.")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tSUBJECT\tTO")
	for _, e := range list.Data {
		to := ""
		if len(e.To) > 0 {
			to = e.To[0]
			if len(e.To) > 1 {
				to = fmt.Sprintf("%s (+%d)", to, len(e.To)-1)
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.ID, e.Status, e.Subject, to)
	}
	return tw.Flush()
}

func runEmailsGet(ctx context.Context, c *client, out io.Writer, id string) error {
	var e email
	if err := c.get(ctx, "/emails/"+id, &e); err != nil {
		return err
	}
	fmt.Fprintf(out, "Message\n  %s\n\n", e.ID)
	fmt.Fprintf(out, "From\n  %s\n\n", e.From)
	fmt.Fprintf(out, "To\n  %v\n\n", e.To)
	fmt.Fprintf(out, "Subject\n  %s\n\n", e.Subject)
	fmt.Fprintf(out, "Status\n  %s\n", e.Status)
	return nil
}

// sendEmailRequest mirrors internal/api's sendEmailRequest JSON shape
// (only the fields this CLI foundation exposes - scheduling/tracking flags
// are left for a later command).
type sendEmailRequest struct {
	From       string            `json:"from"`
	To         []string          `json:"to"`
	Cc         []string          `json:"cc,omitempty"`
	Bcc        []string          `json:"bcc,omitempty"`
	Subject    string            `json:"subject,omitempty"`
	Text       string            `json:"text,omitempty"`
	HTML       string            `json:"html,omitempty"`
	TemplateID string            `json:"template_id,omitempty"`
	Variables  map[string]string `json:"variables,omitempty"`
}

// runEmailsSend implements `mailx-cli emails send`: POST /emails,
// emails:send. Goes through client.postIdempotent so a caller who passed
// --idempotency-key gets the same replay-safety every other client gets
// (spec section 27) - a retry can never double-send.
func runEmailsSend(ctx context.Context, c *client, out io.Writer, req sendEmailRequest, idempotencyKey string) error {
	var e email
	if err := c.postIdempotent(ctx, "/emails", req, idempotencyKey, &e); err != nil {
		return err
	}
	fmt.Fprintf(out, "Accepted %s (status: %s)\n", e.ID, e.Status)
	return nil
}

// runEmailsDiagnose implements `mailx-cli emails diagnose`, the CLI
// workflow the platform spec names explicitly (section 14/16): why did
// this message do what it did, in developer-facing terms - never MailX's
// own internal infrastructure identifiers, matching what
// GET /emails/{id}/events itself already guarantees server-side.
func runEmailsDiagnose(ctx context.Context, c *client, out io.Writer, id string) error {
	var d emailDiagnostics
	if err := c.get(ctx, "/emails/"+id+"/events", &d); err != nil {
		return err
	}
	fmt.Fprintf(out, "Message\n  %s\n\n", d.MessageID)
	fmt.Fprintf(out, "Status\n  %s\n\n", d.Status)

	if len(d.Events) > 0 {
		fmt.Fprintln(out, "Timeline")
		for _, e := range d.Events {
			fmt.Fprintf(out, "  %s  %s\n", e.OccurredAt, e.Type)
		}
		fmt.Fprintln(out)
	}

	for _, a := range d.Attempts {
		fmt.Fprintf(out, "Attempt #%d\n", a.AttemptNumber)
		if a.Recipient != "" {
			fmt.Fprintf(out, "  Recipient       %s\n", a.Recipient)
		}
		fmt.Fprintf(out, "  Decision        %s\n", a.Decision)
		fmt.Fprintf(out, "  Accepted        %v\n", a.Accepted)
		if a.FinalCode != 0 {
			fmt.Fprintf(out, "  SMTP            %d %s\n", a.FinalCode, a.EnhancedStatus)
		}
		if a.RemoteMessage != "" {
			fmt.Fprintf(out, "  Remote message  %s\n", a.RemoteMessage)
		}
		if a.FailureStage != "" {
			fmt.Fprintf(out, "  Failure stage   %s\n", a.FailureStage)
		}
		fmt.Fprintln(out)
	}
	return nil
}
