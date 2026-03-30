# Printing Press Library

The curated collection of CLIs built by the [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).

Every CLI in this library was generated from an API spec, verified through the press's quality gates, and submitted via the `/printing-press publish` skill. They're not wrappers — they have local SQLite sync, offline search, workflow commands, and agent-optimized output.

## Published CLIs

| CLI | Category | API | What it does |
|-----|----------|-----|-------------|
| **[espn-pp-cli](library/media-and-entertainment/espn-pp-cli/)** | Media & Entertainment | ESPN | Sports data — scores, stats, standings, schedules, news, odds across 17 sports and 139 leagues. No API key required. |
| **[linear-pp-cli](library/project-management/linear-pp-cli/)** | Project Management | Linear | Issues, cycles, teams, projects via GraphQL. Local sync, stale detection, team health scoring. |

## Install from the Library

Each CLI is a standalone Go module. You need [Go 1.23+](https://go.dev/dl/) installed.

### go install (recommended)

```bash
# ESPN — sports scores, stats, standings
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn-pp-cli/cmd/espn-pp-cli@latest

# Linear — issues, cycles, team health
go install github.com/mvanhorn/printing-press-library/library/project-management/linear-pp-cli/cmd/linear-pp-cli@latest
```

The binary lands in your `$GOPATH/bin` (or `$HOME/go/bin` by default). Make sure that's on your `PATH`.

### From source

```bash
git clone https://github.com/mvanhorn/printing-press-library.git
cd printing-press-library/library/<category>/<cli-name>
go install ./cmd/<cli-name>
```

Check each CLI's own README for usage, configuration, and required environment variables.

## Structure

```
library/
  <category>/
    <cli-name>/
      cmd/
      internal/
      .printing-press.json    # provenance manifest
      .manuscripts/           # research + verification artifacts
        <run-id>/
          research/
          proofs/
          discovery/
      README.md
      go.mod
      ...
```

CLIs are organized by category. Each CLI folder is self-contained — it includes the full source code, the provenance manifest, and the manuscripts (research briefs, shipcheck results, discovery provenance) from the printing run.

## Categories

| Category | What goes here |
|----------|---------------|
| `developer-tools` | SCM, CI/CD, feature flags, hosting |
| `monitoring` | Error tracking, APM, alerting, product analytics |
| `cloud` | Compute, DNS, CDN, storage, infrastructure |
| `project-management` | Tasks, sprints, issues, roadmaps |
| `productivity` | Docs, wikis, databases, scheduling |
| `social-and-messaging` | Chat, SMS, voice, social, streaming, media |
| `sales-and-crm` | Pipelines, contacts, deals |
| `marketing` | Email campaigns, automation |
| `payments` | Billing, transactions, banking, fintech |
| `auth` | Identity, SSO, user management |
| `commerce` | Storefronts, inventory, orders, shopping |
| `ai` | LLMs, inference, ML, computer vision |
| `devices` | Smart home, wearables, hardware |
| `media-and-entertainment` | Streaming, sports, video, music, content platforms |
| `other` | Anything that doesn't fit above |

## What "Endorsed" Means

Every CLI in this library has passed:

1. **Generation** — Built by the CLI Printing Press from an API spec
2. **Validation** — `go build`, `go vet`, `--help`, and `--version` all pass
3. **Provenance** — `.printing-press.json` manifest and `.manuscripts/` artifacts are present

CLIs may be improved after generation (emboss passes, manual refinements). The manuscripts show what was originally generated, and the diff shows what changed.

## Registry

`registry.json` at the repo root is a machine-readable index of all CLIs:

```json
{
  "schema_version": 1,
  "entries": [
    {
      "cli_name": "espn-pp-cli",
      "api_name": "espn",
      "category": "media-and-entertainment",
      "description": "ESPN sports data CLI — scores, stats, standings across 17 sports",
      "path": "library/media-and-entertainment/espn-pp-cli"
    },
    {
      "cli_name": "linear-pp-cli",
      "api_name": "linear",
      "category": "project-management",
      "description": "Linear project management CLI with offline sync and team health",
      "path": "library/project-management/linear-pp-cli"
    }
  ]
}
```

## Want to generate your own?

The [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) has 18 APIs in its catalog ready to go, and can generate CLIs from any OpenAPI spec, GraphQL schema, or even sniffed browser traffic.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to submit a CLI.

## License

MIT
