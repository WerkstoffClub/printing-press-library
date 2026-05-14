# Songkick CLI

**The Songkick CLI for music-agency tour routing — every documented endpoint, a local SQLite store, and routing intelligence that no wrapper ships.**

Brings every documented Songkick v3.0 endpoint into a single Go binary with offline FTS search, persistent SQLite store, and great-circle distance math. Pairs the documented surface with twelve novel commands (`route`, `gap`, `predict-return`, `reliability`, `saturation`, `venue-tier`, `co-tour`, `route-batch`, `drift`, `promotion`, `lineup-overlap`, `fit`) built for festival programmers who need to answer routing and lineup questions in seconds, not API round-trips.

## Install

The recommended path installs both the `songkick-pp-cli` binary and the `pp-songkick` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install songkick
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install songkick --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/songkick-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-songkick --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-songkick --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-songkick skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-songkick. The skill defines how its required CLI can be installed.
```

## Authentication

Songkick's public API key program is effectively closed to new applicants — bring your own legacy or partner key via the `SONGKICK_API_KEY` environment variable. The CLI never sends the key anywhere except `api.songkick.com/api/3.0`.

## Quick Start

```bash
# set the API key (BYO from a legacy approval)
export SONGKICK_API_KEY=your-key


# verify auth and reachability
songkick-pp-cli doctor


# resolve an artist name to a Songkick ID
songkick-pp-cli find artists --query Radiohead --json --select id,displayName


# fetch upcoming shows for the artist (populates local store)
songkick-pp-cli artists calendar 297938 --json


# score the artist for a Jakarta date
songkick-pp-cli route --artist 297938 --anchor jakarta --on 2026-09-20 --window 14 --json

```

## Unique Features

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

## Usage

Run `songkick-pp-cli --help` for the full command reference and flag list.

## Commands

### artists

Artist calendar, gigography, and similar artists

- **`songkick-pp-cli artists calendar`** - Upcoming shows for an artist
- **`songkick-pp-cli artists gigography`** - Historical performances for an artist
- **`songkick-pp-cli artists similar`** - Artists similar to the given artist

### events

Fetch event details and setlists

- **`songkick-pp-cli events get`** - Get a single event by ID
- **`songkick-pp-cli events setlist`** - Get the setlist for an event (when available)

### find

Search the Songkick API for artists, venues, locations, and events (use `search` for offline FTS over the local store)

- **`songkick-pp-cli find artists`** - Search artists by name
- **`songkick-pp-cli find events`** - Search events by query, artist, location, or date range
- **`songkick-pp-cli find locations`** - Search metro areas / locations by name or coordinates
- **`songkick-pp-cli find venues`** - Search venues by name

### metros

Metro-area calendars (city-level event feeds)

- **`songkick-pp-cli metros calendar`** - Upcoming events in a metro area

### users

User calendars, trackings, and attended events (requires user-scope auth)

- **`songkick-pp-cli users calendar`** - Upcoming events for a user's tracked artists
- **`songkick-pp-cli users events`** - Events a user attended or plans to attend
- **`songkick-pp-cli users trackings`** - Tracked artists, venues, or metro areas for a user

### venues

Venue details and calendar

- **`songkick-pp-cli venues calendar`** - Upcoming events at a venue
- **`songkick-pp-cli venues get`** - Get a venue by ID (includes lat/lng for distance math)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
songkick-pp-cli events get mock-value

# JSON for scripting and agents
songkick-pp-cli events get mock-value --json

# Filter to specific fields
songkick-pp-cli events get mock-value --json --select id,name,status

# Dry run — show the request without sending
songkick-pp-cli events get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
songkick-pp-cli events get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-songkick -g
```

Then invoke `/pp-songkick <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add songkick songkick-pp-mcp -e SONGKICK_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/songkick-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SONGKICK_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "songkick": {
      "command": "songkick-pp-mcp",
      "env": {
        "SONGKICK_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
songkick-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/songkick-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SONGKICK_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `songkick-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SONGKICK_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 401 'Invalid or missing apikey'** — Export SONGKICK_API_KEY with a valid key. Songkick closed new-applicant access ~2017; legacy/partner keys still work.
- **Empty result from `route` for an artist you know is touring** — Run `songkick-pp-cli sync --artist <id>` first — `route` reads from the local store and needs that artist's calendar synced.
- **`page` parameter ignored / only 50 results** — The Songkick API caps `per_page` at 50. The CLI auto-stitches pages; pass `--limit 0` to fetch all pages, or set a higher `--limit`.
- **Stale data after Songkick made site changes** — Songkick was acquired by Suno in Nov 2025; surface changes may happen. Run `songkick-pp-cli doctor` to confirm endpoint reachability.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**python-songkick**](https://github.com/mattdennewitz/python-songkick) — Python (27 stars)
- [**songkick-api-node**](https://github.com/schnogz/songkick-api-node) — JavaScript (8 stars)
- [**songkick-pwa**](https://github.com/zoetrope69/songkick-pwa) — JavaScript (8 stars)
- [**net-songkick**](https://github.com/davorg-cpan/net-songkick) — Perl (5 stars)
- [**Songkick-Unofficial-API**](https://github.com/Integuru-AI/Songkick-Unofficial-API) — Python (1 stars)
- [**tourGEN**](https://github.com/suuhm/tourGEN) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
