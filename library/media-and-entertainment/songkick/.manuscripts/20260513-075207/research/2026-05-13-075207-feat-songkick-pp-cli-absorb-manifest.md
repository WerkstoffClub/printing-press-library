# Songkick CLI — Absorb Manifest

## Scope summary

- **15 absorbed endpoints** from the Songkick v3.0 documented surface — full coverage, beats every existing wrapper.
- **12 transcendence features** — features no existing tool ships, all enabled by the local SQLite store + FTS + geo math.
- **27 total commands.** No existing Songkick tool ships more than ~10 wrapper methods; no existing tool ships any transcendence features. This CLI is the GOAT by construction.
- **BYO-key.** Auth via `SONGKICK_API_KEY` env var. Live testing skipped this run (user has no key; key program closed to new applicants).

## Source tools surveyed

| Tool | URL | Stars | Lang | Last commit | Surface |
|---|---|---|---|---|---|
| songkick-api-node | github.com/schnogz/songkick-api-node | 8 | JS | Jun 2022 | Most complete wrapper |
| python-songkick | github.com/mattdennewitz/python-songkick | 27 | Py | ~2014, dead | events / gigography / setlists |
| Songkick-Unofficial-API | github.com/Integuru-AI/Songkick-Unofficial-API | 1 | Py | recent | Unofficial website scrape |
| songkick-pwa | github.com/zoetrope69/songkick-pwa | 8 | JS | Sep 2020 | UI over API |
| net-songkick | github.com/davorg-cpan/net-songkick | 5 | Perl | Jan 2024 | Perl client |
| tourGEN | github.com/suuhm/tourGEN | 0 | Py | Jun 2025 | Auto tour generator |
| **No Songkick CLI** | — | — | — | — | gap |
| **No Songkick MCP** | — | — | — | — | gap |

---

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|--------------------|-------------|--------|
| 1 | Search artists by query | songkick-api-node `searchArtists` | `songkick search artists "<query>"` + local FTS cache | Offline search after first sync, regex, `--json --select`, agent-native | ship |
| 2 | Search venues by query | songkick-api-node `searchVenues` | `songkick search venues "<query>"` + FTS | Same | ship |
| 3 | Search locations | songkick-api-node `searchLocations` | `songkick search locations "<query>"` | Same | ship |
| 4 | Search events | songkick-api-node `searchEvents` | `songkick search events "<query>" [--type concert\|festival]` | `--type` filter, FTS over cached events | ship |
| 5 | Get event by ID | wrapper `getEvent` | `songkick events get <id>` | `--json`, store cache, `--with-setlist` joins in setlist call | ship |
| 6 | Event setlist | songkick-api-node | `songkick events setlist <id>` | Cached, joinable in SQL | ship |
| 7 | Artist calendar (upcoming shows) | wrapper `getArtistCalendar` | `songkick artists calendar <id> [--mbid]` | Auto page-stitching past 50-cap, persistent store | ship |
| 8 | Artist gigography (historical) | wrapper `getArtistGigography` | `songkick artists gigography <id> [--mbid] [--from YYYY] [--to YYYY]` | Auto page-stitching, date range filter, store-backed | ship |
| 9 | Artist similar artists | (Songkick endpoint, not all wrappers expose) | `songkick artists similar <id>` | `--depth N` for transitive (uses transcendence #8 graph) | ship |
| 10 | Venue details | wrapper `getVenue` | `songkick venues get <id>` | Pulls lat/lng into store for distance math | ship |
| 11 | Venue calendar | wrapper `getVenueCalendar` | `songkick venues calendar <id>` | Page-stitching, store-backed | ship |
| 12 | Metro area calendar | wrapper `getMetroAreaCalendar` | `songkick metros calendar <id>` | Page-stitching, FTS-indexed | ship |
| 13 | User calendar (tracked artists' upcoming) | wrapper `getUserCalendar` | `songkick users calendar <username>` | Requires user-scope auth; standard auth header | ship |
| 14 | User trackings (artists, venues, metros) | wrapper `getUserTracked*` | `songkick users trackings <username> --type artist\|venue\|metro` | Unified subcommand, store-cached | ship |
| 15 | User attended/plan-to-attend events | wrapper `getUserEvents` | `songkick users events <username> [--attendance=i_went\|i_might_go]` | Filter flag, store-cached | ship |

Every Songkick-touching tool surveyed exposes a subset of these endpoints. **We expose all 15** — and every one ships with the agent-native baseline: `--json`, `--select`, `--csv`, `--compact`, `--dry-run`, typed exit codes, SQLite persistence, offline FTS, MCP exposure.

---

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 1 | Tour-routing scan with verdicts | `songkick route --artist <id> --anchor jakarta --on 2026-09-20 --window 14 --radius 5000` | Cross-joins synced artist calendars + great-circle distance + signed day-delta + 60/40 scoring in one SQL pass; API returns raw events only | 10 |
| 2 | Routing-gap detector | `songkick gap --artist <id> --anchor jakarta --window 21` | Finds dates where artist plays in the region but leaves a window unbooked near the anchor; pure local interval math | 9 |
| 3 | Predictive return window | `songkick predict-return --artist <id> --city jakarta` | Mines gigography for median inter-visit interval + seasonality; emits next probable window. Requires multi-year history in store | 9 |
| 4 | Lineup overlap across festivals | `songkick lineup-overlap --events 12345,67890,...` | Joins performances across N events; surfaces shared artists, billing-order deltas. Pure local set algebra | 8 |
| 5 | Cancellation/postponement reliability | `songkick reliability --artist <id> --years 3` | Computes cancel/postpone rate, region clustering, recovery time from local event-status history | 8 |
| 6 | Market saturation heatmap | `songkick saturation --metro <id> --from 2026-08-01 --to 2026-12-31` | Per-week show-count density, capacity-weighted, venue-tier-segmented; answers "is the calendar crowded?" | 8 |
| 7 | Venue-tier classifier | `songkick venue-tier --metro <id>` | Clusters local venues into S/A/B/C tiers via capacity quantiles + headliner-billing frequency + popularity; tier labels persist | 7 |
| 8 | Co-touring artist graph | `songkick co-tour --artist <id> --depth 2` | Builds artist graph from shared bills across gigography; surfaces support-act candidates. Only possible from denormalized performances | 8 |
| 9 | Routing-feasibility batch from shortlist | `songkick route-batch --shortlist artists.csv --anchor jakarta --on 2026-09-20` | Runs 60/40 routing score across 50–200 artists in one SQL query; sorts by verdict, emits NDJSON. API-only = N round-trips | 9 |
| 10 | Drift watcher (snapshot diff) | `songkick drift --since 7d --near jakarta` | Diffs current vs prior — surfaces newly added shows, capacity changes, billing-order promotions, status flips. Needs snapshot history | 8 |
| 11 | Headliner-promotion tracker | `songkick promotion --artist <id>` | Walks gigography ordered by date; detects when billing_index crossed into headliner. Pure local timeline analysis | 8 |
| 12 | Festival-fit scorer | `songkick fit --brief brief.yml` | Brief specifies tier mix + budget caps + genre proxies; scorer joins venue-tier, popularity, routing, co-tour against brief | 9 |

---

## Compound use case (Double Deer end-to-end)

```bash
# 06:00 WIB daily ingest (daily-refresh agent)
songkick sync --artists shortlist.csv --years-back 3 --to-store ./songkick.db

# Tour-router question at 09:00 WIB
songkick route-batch \
  --shortlist memory/active-shortlist.csv \
  --anchor jakarta \
  --on 2026-09-20 \
  --window 14 \
  --radius 5000 \
  --json --select artist,nearestShow,score,verdict \
  | jq '[.[] | select(.verdict == "strong-fit")] | sort_by(-.score)'

# Lineup-planner question: who's also playing nearby?
songkick lineup-overlap --events 12345,67890 --json

# Saturation check: is the date already crowded?
songkick saturation --metro 17681 --from 2026-09-01 --to 2026-10-15
```

No existing Songkick tool can compose these. This is the GOAT.

---

## User Vision

User briefing was "Let's go" — no upfront vision. The implicit vision from the Double Deer project context: feed the existing `tour-router` agent with a typed, cached, scriptable CLI so daily routing-feasibility checks become deterministic and replayable. All transcendence features map directly to that workflow.

---

## Stubs / known limitations

- **No live testing this run.** User has no API key; key program closed. CLI is verified against the documented surface and mock responses only.
- **Features 3 (predict-return) and 5 (reliability) need ≥3 years of synced gigography** to be useful. Implementation ships full algorithm; meaningful output requires the user to `sync` their shortlist first. Documented in README.
- **Drift watcher (#10) requires `snapshot` table** — populated automatically on every `sync` run. Empty until first sync.
- **No XML output** — JSON only. (XML is a documented Songkick format but adds noise without value.)

No stubs in the shipping-scope sense — every row above is a real implementation. The "needs prior sync" caveat applies to data-dependent features and is documented in the help text + README.

---

## Killed candidates

- `songkick search-artist` (wrapped HTTP, 0 transcendence value beyond the absorbed row)
- Genre-similarity recommender (Songkick doesn't expose genre tags reliably; needs MusicBrainz bridge, out of scope)
- Ticket-price tracker (Songkick API doesn't expose prices)
- Demographic overlay (requires Chartmetric/Luminate — separate CLI)
- Social-listening fusion (belongs in social-listener agent)
- User-trackings popularity ranker (requires per-user-scope auth and a corpus we don't have)
- Real-time availability / auto-booking (no such API)
