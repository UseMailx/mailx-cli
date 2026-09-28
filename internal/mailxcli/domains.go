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
// confirmation before doing anything (see runDomainsDelete).
func runDomains(ctx context.Context, c *client, in io.Reader, out io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mailx-cli domains <list|inspect|create|verify|delete> [args]")
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
	default:
		return fmt.Errorf("unknown domains command %q (want list|inspect|create|verify|delete)", args[0])
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
