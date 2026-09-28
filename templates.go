package main

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"
)

// template mirrors internal/api's templateResource JSON shape.
type template struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html"`
}

type templateList struct {
	Data []template `json:"data"`
}

type templatePreview struct {
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html"`
}

// runTemplates dispatches `mailx-cli templates <verb>`. list/get/preview
// only - templates:write (create/update/delete) is deferred, same
// read-first reasoning as domains and emails.
func runTemplates(ctx context.Context, c *client, out io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mailx-cli templates <list|get|preview> [TEMPLATE_ID]")
	}
	switch args[0] {
	case "list":
		return runTemplatesList(ctx, c, out)
	case "get":
		if len(args) != 2 {
			return fmt.Errorf("usage: mailx-cli templates get TEMPLATE_ID")
		}
		return runTemplatesGet(ctx, c, out, args[1])
	case "preview":
		if len(args) != 2 {
			return fmt.Errorf("usage: mailx-cli templates preview TEMPLATE_ID")
		}
		return runTemplatesPreview(ctx, c, out, args[1])
	default:
		return fmt.Errorf("unknown templates command %q (want list|get|preview)", args[0])
	}
}

func runTemplatesList(ctx context.Context, c *client, out io.Writer) error {
	var list templateList
	if err := c.get(ctx, "/templates", &list); err != nil {
		return err
	}
	if len(list.Data) == 0 {
		fmt.Fprintln(out, "No templates.")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tSUBJECT")
	for _, t := range list.Data {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", t.ID, t.Name, t.Subject)
	}
	return tw.Flush()
}

func runTemplatesGet(ctx context.Context, c *client, out io.Writer, id string) error {
	var t template
	if err := c.get(ctx, "/templates/"+id, &t); err != nil {
		return err
	}
	fmt.Fprintf(out, "Template\n  %s (%s)\n\n", t.Name, t.ID)
	fmt.Fprintf(out, "Subject\n  %s\n\n", t.Subject)
	if t.Text != "" {
		fmt.Fprintf(out, "Text\n  %s\n\n", t.Text)
	}
	if t.HTML != "" {
		fmt.Fprintf(out, "HTML\n  %s\n", t.HTML)
	}
	return nil
}

// runTemplatesPreview calls POST /templates/{id}/preview with no
// variables - showing the raw template as stored, including any unfilled
// {{tokens}}. A future `--var key=value` flag can pass real variables
// through; not needed for this foundation.
func runTemplatesPreview(ctx context.Context, c *client, out io.Writer, id string) error {
	var p templatePreview
	if err := c.post(ctx, "/templates/"+id+"/preview", map[string]any{}, &p); err != nil {
		return err
	}
	fmt.Fprintf(out, "Subject\n  %s\n\n", p.Subject)
	if p.Text != "" {
		fmt.Fprintf(out, "Text\n  %s\n\n", p.Text)
	}
	if p.HTML != "" {
		fmt.Fprintf(out, "HTML\n  %s\n", p.HTML)
	}
	return nil
}
