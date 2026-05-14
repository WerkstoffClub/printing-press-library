---
name: pp-songkick
description: "The Songkick CLI for music-agency tour routing — every documented endpoint, a local SQLite store, and routing... Trigger phrases: `find artists near jakarta`, `songkick route check`, `is this artist touring in southeast asia`, `lineup overlap between festivals`, `venue tier classification`, `use songkick-pp-cli`, `run songkick`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - songkick-pp-cli
---

# Songkick — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `songkick-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install songkick --cli-only
   ```
2. Verify: `songkick-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Brings every documented Songkick v3.0 endpoint into a single Go binary with offline FTS search, persistent SQLite store, and great-circle distance math. Pairs the documented surface with twelve novel commands (`route`, `gap`, `predict-return`, `reliability`, `saturation`, `venue-tier`, `co-tour`, `route-batch`, `drift`, `promotion`, `lineup-overlap`, `fit`) built for festival programmers who need to answer routing and lineup questions in seconds, not API round-trips.

## When to Use This CLI

Reach for this CLI when you need to feed a booking team or tour-routing workflow with structured Songkick data — routing-feasibility checks for a target date, shortlist scoring, gigography-derived risk signals, or festival lineup analysis. It assumes you already hold a legacy Songkick API key. Pair with the Bandsintown CLI for cross-source confirmation.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Routing intelligence
- **`route`** — Score an artist for routing feasibility against a target date and anchor city. Returns nearest show, daysFromTarget, distanceKm, score, and a strong-fit / considered / cold verdict.

  _Reach for this when a booking team needs to decide if a target artist is worth a cold pitch or a routing offer. Strong-fit verdict = artist is already in the region._

  ```bash
  songkick-pp-cli route --artist 297938 --anchor jakarta --on 2026-09-20 --window 14 --radius 5000 --json
  ```
- **`gap`** — Find date windows where an artist plays in the region but leaves a slot unbooked near the anchor city — the cheapest pitch opportunity.

  _Use when you want to know not just whether the artist is nearby, but whether there is an open day in their schedule that could absorb a Jakarta date._

  ```bash
  songkick-pp-cli gap --artist 297938 --anchor jakarta --window 21 --json
  ```
- **`route-batch`** — Run the routing-feasibility score across an entire shortlist of artists in one SQL query; sorts by verdict and emits NDJSON for downstream agents.

  _Reach for this when you have a 50-200 artist shortlist and need to rank them by routing economics for a specific event date._

  ```bash
  songkick-pp-cli route-batch --shortlist memory/active-shortlist.csv --anchor jakarta --on 2026-09-20 --window 14 --json
  ```

### Forward-looking intelligence
- **`predict-return`** — Mine an artist's multi-year gigography for median inter-visit interval and seasonality patterns to forecast their next probable window in a given city.

  _Reach for this when planning a multi-quarter calendar; tells you when an artist is overdue for a regional visit._

  ```bash
  songkick-pp-cli predict-return --artist 297938 --city jakarta --json
  ```
- **`drift`** — Diff current snapshot against prior — surface newly added shows near the anchor, capacity changes, billing-order promotions, and status flips since the last sync.

  _Use during the daily-refresh cycle to surface what changed that actually affects the booking team's workflow today._

  ```bash
  songkick-pp-cli drift --since 7d --near jakarta --json
  ```
- **`promotion`** — Walk an artist's gigography ordered by date and detect the inflection point where their billing_index crossed into the headliner slot.

  _Reach for this when scouting emerging artists — surfaces 'about to break' candidates whose billing position is climbing._

  ```bash
  songkick-pp-cli promotion --artist 297938 --json
  ```

### Lineup intelligence
- **`lineup-overlap`** — Compare performances across multiple festival event IDs to surface shared artists, billing-order deltas, and headliner-vs-mid drift.

  _Use during festival programming to check differentiation against competing festivals in the same season._

  ```bash
  songkick-pp-cli lineup-overlap --events 12345,67890,11223 --json
  ```
- **`co-tour`** — Build an artist graph from shared performances; surface support-act candidates and stylistic neighbors out to N hops.

  _Use to find support acts that have already shared bills with the headliner, or to discover stylistic neighbors for an unsigned slot._

  ```bash
  songkick-pp-cli co-tour --artist 297938 --depth 2 --json
  ```
- **`fit`** — Score an artist shortlist against a festival brief (tier mix, budget caps, genre proxies) by joining venue-tier, popularity, routing feasibility, and co-tour adjacency.

  _Reach for this as the final lineup-planner pass — emits ranked artist-to-event fit scores agents can hand to the booking team._

  ```bash
  songkick-pp-cli fit --brief memory/event-brief.yml --shortlist memory/active-shortlist.csv --json
  ```

### Risk intelligence
- **`reliability`** — Compute an artist's cancellation and postponement rate over a multi-year window, with region clustering and recovery-time analysis.

  _Use before signing a deposit-heavy headliner. High cancellation rate = higher booking risk._

  ```bash
  songkick-pp-cli reliability --artist 297938 --years 3 --json
  ```

### Market intelligence
- **`saturation`** — Per-week show-count density for a metro area, capacity-weighted and venue-tier-segmented across a date range.

  _Use to answer 'is this date already crowded?' before committing budget to a competing event._

  ```bash
  songkick-pp-cli saturation --metro 17681 --from 2026-08-01 --to 2026-12-31 --json
  ```
- **`venue-tier`** — Cluster a metro area's venues into S / A / B / C tiers using capacity quantiles, headliner-billing frequency, and average popularity.

  _Reach for this once per metro to label your venue inventory; downstream commands (route, saturation, fit) consume the tier labels._

  ```bash
  songkick-pp-cli venue-tier --metro 17681 --json
  ```

## Command Reference

**artists** — Artist calendar, gigography, and similar artists

- `songkick-pp-cli artists calendar` — Upcoming shows for an artist
- `songkick-pp-cli artists gigography` — Historical performances for an artist
- `songkick-pp-cli artists similar` — Artists similar to the given artist

**events** — Fetch event details and setlists

- `songkick-pp-cli events get` — Get a single event by ID
- `songkick-pp-cli events setlist` — Get the setlist for an event (when available)

**find** — Search the Songkick API for artists, venues, locations, and events (use `search` for offline FTS over the local store)

- `songkick-pp-cli find artists` — Search artists by name
- `songkick-pp-cli find events` — Search events by query, artist, location, or date range
- `songkick-pp-cli find locations` — Search metro areas / locations by name or coordinates
- `songkick-pp-cli find venues` — Search venues by name

**metros** — Metro-area calendars (city-level event feeds)

- `songkick-pp-cli metros <metro_id>` — Upcoming events in a metro area

**users** — User calendars, trackings, and attended events (requires user-scope auth)

- `songkick-pp-cli users calendar` — Upcoming events for a user's tracked artists
- `songkick-pp-cli users events` — Events a user attended or plans to attend
- `songkick-pp-cli users trackings` — Tracked artists, venues, or metro areas for a user

**venues** — Venue details and calendar

- `songkick-pp-cli venues calendar` — Upcoming events at a venue
- `songkick-pp-cli venues get` — Get a venue by ID (includes lat/lng for distance math)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
songkick-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find routing-feasible artists for a Jakarta event

```bash
songkick-pp-cli route-batch --shortlist memory/active-shortlist.csv --anchor jakarta --on 2026-09-20 --window 14 --radius 5000 --json --select artist,nearestShow.date,nearestShow.distanceKm,score,verdict
```

Scores an entire shortlist against a target date and emits NDJSON ready for downstream agents to filter.

### Detect open windows in a target artist's tour

```bash
songkick-pp-cli gap --artist 297938 --anchor jakarta --window 21 --json --select start,end,nearestCity,daysOpen
```

Returns date windows where the artist plays in the region but leaves a slot unbooked near Jakarta — the cheapest pitch opportunity.

### Tier-label every Jakarta venue

```bash
songkick-pp-cli venue-tier --metro 17681 --json --select id,displayName,capacity,tier
```

Classifies Jakarta venues into S/A/B/C tiers and persists the labels for downstream `saturation` and `fit` commands.

### Check festival lineup overlap with a competitor

```bash
songkick-pp-cli lineup-overlap --events 12345,67890 --agent --select sharedArtists,billingDelta
```

Surfaces shared artists between two festivals' performance lists for differentiation analysis.

### Detect what changed near the anchor since last sync

```bash
songkick-pp-cli drift --since 7d --near jakarta --json --select changeType,artist,start.date,venue.displayName
```

Diffs the last week of sync snapshots and emits booking-team-relevant changes only.

## Auth Setup

Songkick's public API key program is effectively closed to new applicants — bring your own legacy or partner key via the `SONGKICK_API_KEY` environment variable. The CLI never sends the key anywhere except `api.songkick.com/api/3.0`.

Run `songkick-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  songkick-pp-cli events get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
songkick-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
songkick-pp-cli feedback --stdin < notes.txt
songkick-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.songkick-pp-cli/feedback.jsonl`. They are never POSTed unless `SONGKICK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SONGKICK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
songkick-pp-cli profile save briefing --json
songkick-pp-cli --profile briefing events get mock-value
songkick-pp-cli profile list --json
songkick-pp-cli profile show briefing
songkick-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `songkick-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add songkick-pp-mcp -- songkick-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which songkick-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   songkick-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `songkick-pp-cli <command> --help`.
