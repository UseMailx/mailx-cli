package main

import (
	"context"
	"flag"
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

// runTemplates dispatches `mailx-cli templates <verb>`.
func runTemplates(ctx context.Context, c *client, in io.Reader, out io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mailx-cli templates <list|get|preview|create|update|delete> [args]")
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
	case "create":
		fs := flag.NewFlagSet("templates create", flag.ContinueOnError)
		name := fs.String("name", "", "template name (required)")
		subject := fs.String("subject", "", "subject, may use {{variables}} (required)")
		text := fs.String("text", "", "plain-text body")
		html := fs.String("html", "", "HTML body")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" || *subject == "" {
			return fmt.Errorf("usage: mailx-cli templates create --name NAME --subject SUBJECT [--text TEXT] [--html HTML]")
		}
		return runTemplatesCreate(ctx, c, out, *name, *subject, *text, *html)
	case "update":
		fs := flag.NewFlagSet("templates update", flag.ContinueOnError)
		name := fs.String("name", "", "new name")
		subject := fs.String("subject", "", "new subject")
		text := fs.String("text", "", "new plain-text body")
		html := fs.String("html", "", "new HTML body")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: mailx-cli templates update [--name N] [--subject S] [--text T] [--html H] TEMPLATE_ID")
		}
		// Only flags the caller actually passed become part of the PATCH
		// body - an unset flag must never silently blank out an existing
		// field (the API itself treats an absent JSON key as "unchanged",
		// this just has to not send the key at all).
		body := map[string]string{}
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "name":
				body["name"] = *name
			case "subject":
				body["subject"] = *subject
			case "text":
				body["text"] = *text
			case "html":
				body["html"] = *html
			}
		})
		if len(body) == 0 {
			return fmt.Errorf("templates update: at least one of --name/--subject/--text/--html is required")
		}
		return runTemplatesUpdate(ctx, c, out, fs.Arg(0), body)
	case "delete":
		fs := flag.NewFlagSet("templates delete", flag.ContinueOnError)
		yes := fs.Bool("yes", false, "skip the confirmation prompt")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: mailx-cli templates delete [--yes] TEMPLATE_ID")
		}
		return runTemplatesDelete(ctx, c, in, out, fs.Arg(0), *yes)
	default:
		return fmt.Errorf("unknown templates command %q (want list|get|preview|create|update|delete)", args[0])
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

// runTemplatesCreate implements `mailx-cli templates create`, templates:write.
func runTemplatesCreate(ctx context.Context, c *client, out io.Writer, name, subject, text, html string) error {
	var t template
	body := map[string]string{"name": name, "subject": subject, "text": text, "html": html}
	if err := c.post(ctx, "/templates", body, &t); err != nil {
		return err
	}
	fmt.Fprintf(out, "Created template %s (%s)\n", t.Name, t.ID)
	return nil
}

// runTemplatesUpdate implements `mailx-cli templates update`, templates:write.
// body carries only the fields the caller actually passed a flag for.
func runTemplatesUpdate(ctx context.Context, c *client, out io.Writer, id string, body map[string]string) error {
	var t template
	if err := c.patch(ctx, "/templates/"+id, body, &t); err != nil {
		return err
	}
	fmt.Fprintf(out, "Updated template %s (%s)\n", t.Name, t.ID)
	return nil
}

// runTemplatesDelete implements `mailx-cli templates delete`, templates:write,
// irreversible (v0.33: hard delete, no versioning) - confirmed first unless
// --yes, same guard as domains delete.
func runTemplatesDelete(ctx context.Context, c *client, in io.Reader, out io.Writer, id string, skipConfirm bool) error {
	ok, err := confirm(in, out, fmt.Sprintf("Delete template %s? This cannot be undone.", id), skipConfirm)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(out, "Aborted.")
		return nil
	}
	if err := c.deleteReq(ctx, "/templates/"+id); err != nil {
		return err
	}
	fmt.Fprintf(out, "Deleted template %s.\n", id)
	return nil
}
