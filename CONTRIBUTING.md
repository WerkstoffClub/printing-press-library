# Contributing to CLI-PP-Library

## Submitting a CLI

### Prerequisites

1. Install the [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
2. Generate your CLI: `/printing-press <API-name>`
3. Verify it passes quality gates

### Quality Gates (All Required)

```bash
# 1. Runtime verification - must pass at 80%+
printing-press verify --dir ./your-cli --spec your-spec.json

# 2. Scorecard - must score 75+
printing-press scorecard --dir ./your-cli

# 3. Build verification
cd your-cli && go build ./... && go vet ./...
```

### PR Checklist

- [ ] CLI source code in its own directory (e.g., `your-api-cli/`)
- [ ] OpenAPI spec used to generate it
- [ ] Phase 0-5 plan artifacts in `docs/plans/`
- [ ] Verify report output (copy from terminal)
- [ ] Scorecard report output
- [ ] README with product thesis (who is this for, why it matters)
- [ ] At least one emboss pass applied

### Commit Style

Conventional commits: `feat(api-name):`, `fix(api-name):`, `docs(api-name):`

### What Makes a Good Submission

- The CLI solves a real problem (not just wraps endpoints)
- The data layer works end to end (sync -> sql -> search)
- Workflow commands are named after user outcomes, not API resources
- The README tells a story, not just lists commands
- Live API testing passes (if you have an API key)
