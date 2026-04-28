# dotenv-doctor

**The dashboard for `.env` files you've lost track of — with built-in leak detection.**

Every developer has 20 `.env` files scattered across projects, zero idea what's in them,
and a recurring fear they committed one to git. `envs` gives you a port-whisperer-style
table of every project, every key, and every leak — in one local, dependency-free CLI.

```text
$ envs

╭──────────────────────────────────────────────────╮
│  envs · dotenv-doctor                            │
│  the .env files you've lost track of             │
╰──────────────────────────────────────────────────╯

┌──────────────┬───────────┬──────┬──────┬─────────┬────────────────────────────────┐
│ PROJECT      │ FRAMEWORK │ FILE │ KEYS │ MISSING │ ISSUES                         │
├──────────────┼───────────┼──────┼──────┼─────────┼────────────────────────────────┤
│ frontend     │ Next.js   │ .env │ 14   │ 2       │ ! AWS_SECRET committed once    │
│ backend-api  │ Express   │ .env │ 31   │ 0       │ —                              │
│ scratch      │ —         │ .env │ 8    │ —       │ no .env.example                │
│ side-project │ Next.js   │ .env │ 22   │ 5       │ ! exposed secret in client     │
└──────────────┴───────────┴──────┴──────┴─────────┴────────────────────────────────┘

  4 projects  ·  1 leak  ·  7 missing keys  ·  envs <project> for detail
```

## Install

```bash
# macOS / Linux via Homebrew
brew install zigamedved/tap/dotenv-doctor

# any platform with Go
go install github.com/zigamedved/dotenv-doctor/cmd/envs@latest
```

## Usage

```bash
envs                       # the dashboard
envs <project>             # detail view: every key, masked, with notes
envs leaks                 # scan git history for committed .env files
envs check                 # CI-friendly: exit 1 if any project missing keys vs .env.example
envs init                  # one-time setup wizard
```

### `envs leaks` — the moment everyone shares

```text
$ envs leaks

▾ /Users/me/code/frontend
┌──────┬─────────┬────────────┬───────────────────────────┬──────────────────────┬──────────┐
│ PATH │ COMMIT  │ DATE       │ AUTHOR                    │ FINDINGS             │ AT HEAD  │
├──────┼─────────┼────────────┼───────────────────────────┼──────────────────────┼──────────┤
│ .env │ a3f2b91 │ 2024-03-12 │ me <me@example.com>       │ AWS Access Key ID    │ no       │
└──────┴─────────┴────────────┴───────────────────────────┴──────────────────────┴──────────┘
  remediation:
    File is no longer at HEAD but remains in history.
      Rotate the leaked credentials immediately.
      Rewrite history: git filter-repo --path .env --invert-paths
  GitHub secret scanning: https://github.com/me/frontend/security/secret-scanning
```

## What it actually does

- **Discovers** every `.env`-style file under your scan paths (skipping
  `node_modules`, `.git`, `vendor`, `dist`, `build`, etc.) up to a configurable depth.
- **Parses** them — handles quoted values, multiline strings, escapes, `export` prefixes,
  and trailing comments.
- **Classifies** values against a curated set of secret patterns (AWS, GitHub, Stripe,
  OpenAI, Anthropic, Google, Slack, Discord, Twilio, JWTs, PEM headers, ...).
- **Detects frameworks** from `package.json` / `requirements.txt` / `manage.py` and
  flags secrets exposed in `NEXT_PUBLIC_*` / `VITE_*` / `REACT_APP_*` — the most
  common production footgun.
- **Walks git history** with pure-Go go-git (no system git binary needed) to find
  any `.env` ever committed, with the commit SHA, author, and remediation steps.
- **Compares** against `.env.example` to flag missing keys per project, suitable
  for CI via `envs check`.

## Privacy

- **Zero network calls.** Ever. Everything stays local.
- **Mask by default.** Values display as `AW••••••••EY`. Unmasking via `--reveal`
  requires you to type the project name as confirmation.
- **Read-only v0.1.** No writes anywhere except `~/.config/envs/config.toml`,
  and only via the explicit `envs init` wizard.
- **No telemetry.** None. Audit the source — it's small.

## Comparison

|                            | dotenv-doctor      | gitleaks       | dotenv-linter  | Doppler / Phase |
| -------------------------- | ------------------ | -------------- | -------------- | --------------- |
| Glanceable dashboard       | yes                | no             | no             | yes (web)       |
| Built for developers       | yes                | security teams | yes            | yes             |
| Leak detection in history  | yes                | yes (deeper)   | no             | n/a             |
| Framework-aware exposure   | yes                | no             | no             | partial         |
| Replaces `.env` files      | no                 | no             | no             | yes             |
| Local-only / no account    | yes                | yes            | yes            | no (paid SaaS)  |
| Single static binary       | yes                | yes            | yes            | no              |

`dotenv-doctor` fills the gap between "security scanner" and "secret manager":
a friendly developer dashboard for the `.env` files you actually have.

## Configuration

Optional. Stored at `~/.config/envs/config.toml` (or `$XDG_CONFIG_HOME/envs/config.toml`):

```toml
scan_paths = ["~/code", "~/work"]
max_depth  = 4
skip_dirs  = ["legacy-monorepo"]
```

If no config exists and you run `envs` from outside a git project, the
`init` wizard runs automatically.

## CI usage

```yaml
# .github/workflows/check-envs.yml
- run: |
    go install github.com/zigamedved/dotenv-doctor/cmd/envs@latest
    envs check --path . --strict
```

Exit codes:

- `0` — all projects in sync with their `.env.example`
- `1` — at least one project is missing keys
- `2` — internal error

## Development

```bash
git clone https://github.com/zigamedved/dotenv-doctor.git
cd dotenv-doctor
go build ./...
go test ./...
```

The codebase is small and well-tested (~65 unit tests across the parsing,
discovery, classification, and git modules). PRs welcome — keep the rule set
curated and the scope narrow.

## License

MIT. See [LICENSE](LICENSE).
