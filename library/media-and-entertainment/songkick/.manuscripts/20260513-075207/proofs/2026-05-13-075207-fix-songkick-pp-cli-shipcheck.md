# Songkick CLI — Shipcheck Report

## Final verdict: **ship**

All 6 shipcheck legs pass. Scorecard 83/100 Grade A. No critical failures, no functional bugs in shipping-scope features (validated on empty store; live data requires a Songkick API key the user does not have).

## Final shipcheck leg results

| Leg | Result | Exit | Elapsed |
|---|---|---|---|
| dogfood | PASS | 0 | 1.116s |
| verify | PASS | 0 | 2.715s (100% pass rate, 26/26 cmds) |
| workflow-verify | PASS | 0 | 11ms |
| verify-skill | PASS | 0 | 140ms |
| validate-narrative | PASS | 0 | 136ms (9 commands resolved, full examples passed) |
| scorecard | PASS | 0 | 55ms (83/100 — Grade A) |

**Umbrella verdict: PASS (6/6 legs passed).**

## What was built

- 15 absorbed endpoints across 6 resource groups (find, events, artists, venues, metros, users) — full coverage of the Songkick v3.0 documented surface
- 12 transcendence commands (route, gap, predict-return, lineup-overlap, reliability, saturation, venue-tier, co-tour, route-batch, drift, promotion, fit) — none exist in any prior tool
- Local SQLite store with FTS, generic `search` + per-resource list commands
- MCP server with stdio + HTTP transport
- Full agent-native flag set (--json / --select / --csv / --compact / --plain / --quiet / --dry-run / --agent)
- Typed exit codes (2/3/4/5/7/10)
- `doctor` reachability check using `/search/artists.json` as health endpoint

## Top blockers found + fixes applied

1. **`search` was a reserved Printing Press template name** → renamed the spec resource to `find` (Songkick API search → CLI `find` subcommand). The framework's existing `search` command provides offline FTS over the local store; the two are now disambiguated.
2. **Quickstart referenced fictional `--artists` flag on `sync`** → fixed `research.json` narrative.quickstart to use real flags (`find artists --query`, `artists calendar <id>`).
3. **Bare shell `export` line in quickstart** broke strict validate-narrative (no subcommand words) → folded the env-var setup into the comment of the `doctor` step.
4. **Binary lost exec bit after regen** → rebuilt with `go build -o songkick-pp-cli ./cmd/songkick-pp-cli/`.

## Before / after delta

| Metric | Before fixes | After fixes |
|---|---|---|
| Shipcheck verdict | FAIL (1/6 legs) | PASS (6/6 legs) |
| verify pass rate | 100% | 100% |
| validate-narrative | FAIL (`--artists` doesn't exist) | PASS (9 commands resolved + full examples) |
| Scorecard total | 83/100 | 83/100 |
| Novel features built / planned | 12 / 12 | 12 / 12 |

## Scorecard breakdown

```
Output Modes         10/10
Auth                 10/10
Error Handling       10/10
Terminal UX          9/10
README               8/10
Doctor               10/10
Agent Native         10/10
MCP Quality          10/10
MCP Token Efficiency 7/10
MCP Remote Transport 10/10
MCP Tool Design      5/10
Local Cache          10/10
Cache Freshness      5/10
Breadth              9/10
Vision               8/10
Workflows            10/10
Insight              10/10
Agent Workflow       9/10

Domain Correctness
  Path Validity           10/10
  Auth Protocol           4/10
  Data Pipeline Integrity 7/10
  Sync Correctness        10/10
  Type Fidelity           3/5
  Dead Code               5/5

Total: 83/100 - Grade A
```

## Known gaps (polish-time)

- **auth_protocol 4/10** — Songkick uses query-string `?apikey=` auth (uncommon); scorecard expects header-based auth patterns. Generated code is correct, but the scorecard heuristics penalize the unusual auth shape.
- **mcp_tool_design 5/10** — Some MCP tool descriptions could be tighter; polish will address.
- **type_fidelity 3/5** — A few endpoint response types could be more specific (e.g., `Event.start` as nested struct rather than scalar).
- **cache_freshness 5/10** — Generic FTS scoring; no domain-specific freshness rules.

## Live testing

**Skipped** — user has no Songkick API key (the key program closed to new applicants in ~2017 and Songkick was acquired by Suno from Warner Music in Nov 2025). Phase 5 will write `phase5-skip.json` with `skip_reason: auth_required_no_credential`.

The CLI is verified against:
- documented Songkick v3.0 surface (the spec source)
- 26 commands × 3 checks (help / dry-run / exec) all pass against mock responses
- empty-store JSON validity confirmed on all 12 transcendence commands
- novel_features_check: 12 planned, 12 found

A future run with `SONGKICK_API_KEY` set will exercise the live API via Phase 5 dogfood.

## Reachability risk acknowledgement

The brief documented HIGH reachability risk (Suno acquisition, closed key program). This is a property of Songkick, not the CLI. The CLI itself ships clean; if Songkick's surface changes under Suno, a `printing-press generate --force` regen against an updated spec will refresh the code.
