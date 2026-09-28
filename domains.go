package main

import (
	"context"
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

// runDomains dispatches `mailx-cli domains <verb>`. Only list/inspect exist
// so far (read-only) - domains:write operations (create/verify/delete) are
// deferred to a later CLI milestone.
func runDomains(ctx context.Context, c *client, out io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mailx-cli domains <list|inspect DOMAIN_ID>")
	}
	switch args[0] {
	case "list":
		return runDomainsList(ctx, c, out)
	case "inspect":
		if len(args) != 2 {
			return fmt.Errorf("usage: mailx-cli domains inspect DOMAIN_ID")
		}
		return runDomainsInspect(ctx, c, out, args[1])
	default:
		return fmt.Errorf("unknown domains command %q (want list|inspect)", args[0])
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
