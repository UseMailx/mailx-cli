package main

import (
	"context"
	"fmt"
	"io"
)

// whoamiResponse mirrors internal/api's whoamiResource JSON shape. The CLI
// keeps its own copy rather than importing the server package: a client
// binary depends on the API's public JSON contract, never on internal Go
// types (spec section 2 - no interface-specific backend logic, and that
// cuts both ways).
type whoamiResponse struct {
	Organization struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"organization"`
	APIKey struct {
		ID     string   `json:"id"`
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	} `json:"api_key"`
}

// runWhoami implements `mailx-cli whoami`: GET /whoami, printed as the
// spec's own worked example formats it - organization, credential
// identity, granted scopes. Never prints anything beyond what the API
// itself already returns (no raw secret exists to print).
func runWhoami(ctx context.Context, c *client, out io.Writer) error {
	var resp whoamiResponse
	if err := c.get(ctx, "/whoami", &resp); err != nil {
		return err
	}
	fmt.Fprintln(out, "Authenticated")
	fmt.Fprintf(out, "  Organization    %s\n", resp.Organization.Name)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Credential")
	fmt.Fprintf(out, "  %s\n", resp.APIKey.ID)
	fmt.Fprintf(out, "  %s\n", resp.APIKey.Name)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Scopes")
	if len(resp.APIKey.Scopes) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	for _, s := range resp.APIKey.Scopes {
		fmt.Fprintf(out, "  %s\n", s)
	}
	return nil
}
