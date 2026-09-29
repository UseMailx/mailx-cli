package mailxcli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"
)

// domain mirrors internal/api's domainResource JSON shape closely enough
// for display - the CLI depends on the public API contract, not on
// internal Go types (see main.go's package doc).
type domain struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	OwnershipState string  `json:"ownership_state"`
	VerifiedAt     *string `json:"verified_at"`
	Records        []struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"records"`
}

type domainList struct {
	Data []domain `json:"data"`
}

// runDomains dispatches `mailx-cli domains <verb>`. create/verify/delete are
// execute-tier (spec section 11: they produce a real side effect - a new
// domain row, a DNS verification attempt, or an irreversible deletion), so
// unlike list/inspect they never happen implicitly and delete asks for
// confirmation before doing anything (see runDomainsDelete). dkim/spf/
// dmarc/bimi are their own nested verb groups (`domains dkim get ID`, etc)
// since each is a distinct DNS authentication mechanism on the domain, not
// a single flat operation.
func runDomains(ctx context.Context, c *client, in io.Reader, out io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mailx-cli domains <list|inspect|create|verify|delete|dkim|spf|dmarc|bimi> [args]")
	}
	switch args[0] {
	case "list":
		return runDomainsList(ctx, c, out)
	case "inspect":
		if len(args) != 2 {
			return fmt.Errorf("usage: mailx-cli domains inspect DOMAIN_ID")
		}
		return runDomainsInspect(ctx, c, out, args[1])
	case "create":
		if len(args) != 2 {
			return fmt.Errorf("usage: mailx-cli domains create DOMAIN_NAME")
		}
		return runDomainsCreate(ctx, c, out, args[1])
	case "verify":
		if len(args) != 2 {
			return fmt.Errorf("usage: mailx-cli domains verify DOMAIN_ID")
		}
		return runDomainsVerify(ctx, c, out, args[1])
	case "delete":
		fs := flag.NewFlagSet("domains delete", flag.ContinueOnError)
		yes := fs.Bool("yes", false, "skip the confirmation prompt")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: mailx-cli domains delete [--yes] DOMAIN_ID")
		}
		return runDomainsDelete(ctx, c, in, out, fs.Arg(0), *yes)
	case "dkim":
		return runDomainsDKIM(ctx, c, out, args[1:])
	case "spf":
		return runDomainsSPF(ctx, c, out, args[1:])
	case "dmarc":
		return runDomainsDMARC(ctx, c, out, args[1:])
	case "bimi":
		return runDomainsBIMI(ctx, c, out, args[1:])
	default:
		return fmt.Errorf("unknown domains command %q (want list|inspect|create|verify|delete|dkim|spf|dmarc|bimi)", args[0])
	}
}

func runDomainsList(ctx context.Context, c *client, out io.Writer) error {
	var list domainList
	if err := c.get(ctx, "/domains", &list); err != nil {
		return err
	}
	if len(list.Data) == 0 {
		fmt.Fprintln(out, "No domains.")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tSTATUS")
	for _, d := range list.Data {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", d.ID, d.Name, d.OwnershipState)
	}
	return tw.Flush()
}

func runDomainsInspect(ctx context.Context, c *client, out io.Writer, id string) error {
	var d domain
	if err := c.get(ctx, "/domains/"+id, &d); err != nil {
		return err
	}
	fmt.Fprintf(out, "Domain\n  %s (%s)\n\n", d.Name, d.ID)
	fmt.Fprintf(out, "Status\n  %s\n", d.OwnershipState)
	if d.VerifiedAt != nil {
		fmt.Fprintf(out, "Verified at\n  %s\n", *d.VerifiedAt)
	}
	if len(d.Records) > 0 {
		fmt.Fprintln(out, "\nDNS records")
		for _, r := range d.Records {
			fmt.Fprintf(out, "  %s  %s  %s\n", r.Type, r.Name, r.Value)
		}
	}
	return nil
}

// runDomainsCreate implements `mailx-cli domains create`: POST /domains,
// domains:write. The response already carries the DNS records the caller
// needs to publish before verify will succeed, so this prints them
// immediately rather than requiring a follow-up inspect.
func runDomainsCreate(ctx context.Context, c *client, out io.Writer, name string) error {
	var d domain
	if err := c.post(ctx, "/domains", map[string]any{"name": name}, &d); err != nil {
		return err
	}
	fmt.Fprintf(out, "Created domain %s (%s), status %s\n", d.Name, d.ID, d.OwnershipState)
	if len(d.Records) > 0 {
		fmt.Fprintln(out, "\nPublish this DNS record, then run: mailx-cli domains verify", d.ID)
		for _, r := range d.Records {
			fmt.Fprintf(out, "  %s  %s  %s\n", r.Type, r.Name, r.Value)
		}
	}
	return nil
}

// runDomainsVerify implements `mailx-cli domains verify`: POST
// /domains/{id}/verify, domains:write. A failed verification is not a CLI
// error - it is useful output (the DNS record is not visible yet), so the
// server's own response is what is shown, not a generic failure.
func runDomainsVerify(ctx context.Context, c *client, out io.Writer, id string) error {
	var d domain
	if err := c.post(ctx, "/domains/"+id+"/verify", nil, &d); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s: %s\n", d.Name, d.OwnershipState)
	return nil
}

// runDomainsDelete implements `mailx-cli domains delete`: DELETE
// /domains/{id}, domains:write, irreversible - confirmed first unless
// --yes was given (spec section 25: no interface may turn ambiguity into
// an uncontrolled side effect, and a CLI running non-interactively in a
// script is exactly the case --yes exists for).
func runDomainsDelete(ctx context.Context, c *client, in io.Reader, out io.Writer, id string, skipConfirm bool) error {
	ok, err := confirm(in, out, fmt.Sprintf("Delete domain %s? This cannot be undone.", id), skipConfirm)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(out, "Aborted.")
		return nil
	}
	if err := c.deleteReq(ctx, "/domains/"+id); err != nil {
		return err
	}
	fmt.Fprintf(out, "Deleted domain %s.\n", id)
	return nil
}

// authRecordStatus mirrors the shape GET .../dkim|spf|dmarc|bimi returns:
// a status plus the DNS record(s) to publish. Printed generically since
// each mechanism's own extra fields (e.g. DKIM's selector) vary and the
// API is the source of truth for what's actually returned.
func printAuthStatus(out io.Writer, label string, status map[string]any) {
	fmt.Fprintf(out, "%s\n", label)
	for k, v := range status {
		fmt.Fprintf(out, "  %s: %v\n", k, v)
	}
}

func runDomainsDKIM(ctx context.Context, c *client, out io.Writer, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: mailx-cli domains dkim <get|create|verify> DOMAIN_ID")
	}
	id := args[1]
	var status map[string]any
	switch args[0] {
	case "get":
		if err := c.get(ctx, "/domains/"+id+"/dkim", &status); err != nil {
			return err
		}
	case "create":
		// Generates or rotates the domain's DKIM key - an execute-tier
		// operation with a real, irreversible effect (the old key stops
		// signing), but not destructive in the delete sense, so no
		// confirmation prompt: this mirrors "create/rotate an API key",
		// which the CLI also doesn't confirm.
		if err := c.post(ctx, "/domains/"+id+"/dkim", nil, &status); err != nil {
			return err
		}
	case "verify":
		if err := c.post(ctx, "/domains/"+id+"/dkim/verify", nil, &status); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown domains dkim command %q (want get|create|verify)", args[0])
	}
	printAuthStatus(out, "DKIM", status)
	return nil
}

func runDomainsSPF(ctx context.Context, c *client, out io.Writer, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: mailx-cli domains spf <get|verify> DOMAIN_ID")
	}
	id := args[1]
	var status map[string]any
	switch args[0] {
	case "get":
		if err := c.get(ctx, "/domains/"+id+"/spf", &status); err != nil {
			return err
		}
	case "verify":
		if err := c.post(ctx, "/domains/"+id+"/spf/verify", nil, &status); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown domains spf command %q (want get|verify)", args[0])
	}
	printAuthStatus(out, "SPF", status)
	return nil
}

func runDomainsDMARC(ctx context.Context, c *client, out io.Writer, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: mailx-cli domains dmarc <get|verify> DOMAIN_ID")
	}
	id := args[1]
	var status map[string]any
	switch args[0] {
	case "get":
		if err := c.get(ctx, "/domains/"+id+"/dmarc", &status); err != nil {
			return err
		}
	case "verify":
		if err := c.post(ctx, "/domains/"+id+"/dmarc/verify", nil, &status); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown domains dmarc command %q (want get|verify)", args[0])
	}
	printAuthStatus(out, "DMARC", status)
	return nil
}

func runDomainsBIMI(ctx context.Context, c *client, out io.Writer, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: mailx-cli domains bimi <get|verify> DOMAIN_ID")
	}
	id := args[1]
	var status map[string]any
	switch args[0] {
	case "get":
		if err := c.get(ctx, "/domains/"+id+"/bimi", &status); err != nil {
			return err
		}
	case "verify":
		if err := c.post(ctx, "/domains/"+id+"/bimi/verify", nil, &status); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown domains bimi command %q (want get|verify)", args[0])
	}
	printAuthStatus(out, "BIMI", status)
	return nil
}
