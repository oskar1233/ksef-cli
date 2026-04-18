# ksef

Small Go CLI for working with the Polish KSeF API.

This repository is published as a working reference project and personal PoC. It is not a polished end-user product. PRs are not being accepted.

## What it does

The CLI covers the bootstrap auth flow and the regular invoice flow:

- initialize local settings
- request an auth challenge and generate unsigned XML
- submit a signed XAdES XML document
- poll auth status and redeem access and refresh tokens
- generate and store a reusable KSeF token
- authenticate with the saved KSeF token
- list invoice metadata
- download invoice XML files
- render invoice PDFs
- export invoice metadata to CSV

Local settings and tokens are stored in `~/.ksef/settings.json`.

## Build and install

Build a local binary:

```bash
go build -o bin/ksef .
```

Run tests:

```bash
go test ./...
```

Install the binary and shell completions:

```bash
make install
```

By default `make install` writes:

- the binary to `~/.local/bin/ksef`
- bash completion to `~/.local/share/bash-completion/completions/ksef`
- zsh completion to `~/.local/share/zsh/site-functions/_ksef`
- fish completion to `~/.config/fish/completions/ksef.fish`

You can override those paths:

```bash
make install INSTALL_DIR=$HOME/bin
make install ZSH_COMPLETION_DIR=$HOME/.zsh/completions
```

For zsh, make sure `~/.local/share/zsh/site-functions` is on `fpath` and `compinit` is enabled.

Generate completion files without installing them:

```bash
make completions
```

You can also use Cobra directly:

```bash
ksef completion bash
ksef completion zsh
ksef completion fish
```

## Configuration

Initialize or update local settings:

```bash
ksef init --nip <NIP> --environment test
```

The CLI keeps all local path defaults in `~/.ksef/settings.json`. You can set them during `init` and still override them per command later.

Relevant `init` flags:

- `--nip`
- `--environment demo|test|production`
- `--base-url`
- `--subject-identifier-type certificateSubject|certificateFingerprint`
- `--verify-certificate-chain`
- `--auth-request-file`
- `--signed-auth-request-file`
- `--download-dir`
- `--pdf-dir`
- `--export-dir`

Default local paths are:

- unsigned auth XML: `./auth_request.xml`
- signed auth XML: `./signed_auth_request.xml`
- invoice XML downloads: `./invoices`
- rendered PDFs: `./invoice-pdfs`
- CSV exports: `./exports`

Example:

```bash
ksef init \
  --nip <NIP> \
  --environment test \
  --download-dir "$HOME/Documents/ksef/xml" \
  --pdf-dir "$HOME/Documents/ksef/pdfs" \
  --export-dir "$HOME/Documents/ksef/csv"
```

The code defaults to the `demo` environment unless you set another one.

## Auth flow

The CLI supports two auth paths.

### 1. Bootstrap auth with signed XAdES XML

Use this once to get short-lived access and refresh tokens. Then generate a reusable KSeF token.

Step 1. Request a challenge and write unsigned XML:

```bash
ksef challenge
```

Or override the output path explicitly:

```bash
ksef challenge -o /tmp/auth_request.xml
```

Step 2. Sign the XML outside the CLI. The project does not sign XAdES documents itself. You need to sign the generated XML with your certificate or trusted signing tool and produce a signed XML file.

Step 3. Submit the signed XML:

```bash
ksef authorize
```

Or override the input path explicitly:

```bash
ksef authorize -f /tmp/signed_auth_request.xml
```

Step 4. Poll auth status until it completes:

```bash
ksef get-auth-status --wait
```

Step 5. Redeem the auth token for access and refresh tokens:

```bash
ksef redeem
```

Step 6. Generate a reusable KSeF token for later use:

```bash
ksef generate-token
```

Useful auth flags:

```bash
ksef authorize --verify-certificate-chain
ksef get-auth-status --wait --timeout 60s
ksef redeem --authenticationToken <token>
```

### 2. Regular auth with saved KSeF token

After `generate-token`, the CLI can authenticate with the saved token and refresh access automatically.

You can run the token auth flow directly:

```bash
ksef token-auth
```

Or just run a normal command. If the access token is missing or expired, the CLI falls back to refresh or token auth automatically.

You can also force a refresh explicitly:

```bash
ksef refresh
```

## Common usage

List purchase invoices for a month:

```bash
ksef list-invoices -m 2026-04
ksef list-invoices -m 2026-04 -o json
ksef list-invoices -m 2026-04 -o csv
```

List last month for purchase and sales:

```bash
ksef list-last-month --subject both
```

Download invoice XML files. The command uses the configured default download directory unless you pass `--dir`:

```bash
ksef download -m 2026-04
ksef download -m 2026-04 -d ./invoices
```

Render PDFs. The command uses the configured default PDF directory unless you pass `--dir`:

```bash
ksef download-pdfs -m 2026-04 --subject both
ksef download-pdfs -m 2026-04 --subject both -d ./invoice-pdfs
ksef download-last-month-pdfs --subject purchase
```

Export invoice metadata to CSV. The command uses the configured default export directory unless you pass `--dir`:

```bash
ksef export-csv -m 2026-04 --subject both
ksef export-csv -m 2026-04 --subject both -d ./exports
```

## PDF rendering

PDF rendering uses Chrome or Chromium through `chromedp`.

If Chrome is not found automatically, set one of these environment variables:

```bash
export KSEF_CHROME_PATH=/path/to/chrome
# or
export CHROME_PATH=/path/to/chrome
```

## Generated local outputs

The tool writes these local outputs by default:

- `./invoices/`
- `./invoice-pdfs/`
- `./exports/`
- `./bin/` when you build locally

The repository `.gitignore` covers those default generated output locations and `.DS_Store`.

The default auth XML filenames `./auth_request.xml` and `./signed_auth_request.xml` are ignored by the repository `.gitignore`.

Custom auth XML paths are user-managed and are not ignored automatically.

## Code map

Start here if you want to inspect the implementation.

`main.go` wires the Cobra CLI and registers all commands.

`internal/settings/settings.go` handles local settings persistence in `~/.ksef/settings.json`, including configurable output paths.

`internal/ksef.go` contains the KSeF client, request and response types, XML generation, token encryption, and invoice download helpers.

The XAdES bootstrap auth path lives in these files:

- `commands/challenge.go`
- `commands/authorize.go`
- `commands/get_auth_status.go`
- `commands/redeem.go`

The reusable KSeF token path lives in these files:

- `commands/generate_token.go`
- `commands/token_auth.go`
- `commands/refresh.go`
- `commands/common.go`

Invoice listing and export logic lives in:

- `commands/list_invoices.go`
- `commands/list_last_month.go`
- `commands/download.go`
- `commands/download_pdfs.go`
- `commands/export_csv.go`
- `internal/render/`

## Safety notes

Before publishing or sharing a working tree, remove any local XML, PDF, CSV, and binary artifacts. Do not publish real NIP values, signed XML files, access tokens, refresh tokens, KSeF tokens, or downloaded invoices.
