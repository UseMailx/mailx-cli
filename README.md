# mailx-cli

The MailX developer CLI: a thin client over the [MailX](https://github.com/Ferousco-dev/mailx) Product API — the same
capability layer every SDK and the [MCP server](https://github.com/UseMailx/mailx-mcp) use. It contains no business
logic of its own: every command is an HTTP call to a documented `/v1` route.

## Install

With Go installed:

```bash
go install github.com/UseMailx/mailx-cli/cmd/mailx-cli@latest
```

This installs a `mailx-cli` binary onto your `$GOPATH/bin` (or `$GOBIN`).

Prebuilt binaries (macOS, Linux, Windows) are attached to each [release](https://github.com/UseMailx/mailx-cli/releases)
for anyone without a Go toolchain.

## Configure

```bash
export MAILX_API_KEY=mx_...
export MAILX_API_BASE_URL=https://api.mailx.dev/v1   # optional; defaults to this. Point it at your
                                                       # self-hosted deployment instead if you run one.
```

## Usage

```
mailx-cli whoami

mailx-cli domains   list | inspect | create | verify | delete [--yes]
mailx-cli domains   dkim <get|create|verify> DOMAIN_ID
mailx-cli domains   spf|dmarc <get|verify> DOMAIN_ID
mailx-cli domains   bimi <get|verify> DOMAIN_ID
mailx-cli templates list | get | preview | create | update | delete [--yes]
mailx-cli emails    list | get | diagnose | send
```

Run `mailx-cli --help` for the full command list.

### Example

```bash
mailx-cli whoami

mailx-cli domains create example.com
mailx-cli domains verify DOMAIN_ID

mailx-cli templates create --name welcome --subject "Welcome, {{name}}" --text "Hi {{name}}!"
mailx-cli templates preview TEMPLATE_ID

mailx-cli emails send --from hello@example.com --to dest@example.com \
  --template-id TEMPLATE_ID --var name=Ada \
  --idempotency-key my-unique-key-1
```

## License

MIT
