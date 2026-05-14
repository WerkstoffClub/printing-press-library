# Songkick CLI Brief

## API Identity
- Domain: Live music / concert discovery (events, artists, venues, metro areas, calendars)
- Owner: Acquired by Suno (AI music) from Warner Music Group in Nov 2025; 25-person team eliminated, future uncertain
- Users: Music agencies, festival promoters, tour-routing engineers, gig aggregators
- Data profile: Events (id, type=Concert|Festival, displayName, start/end, popularity, status), Artists (id, displayName, identifier MBID), Venues (id, name, capacity, lat/lng), Metro areas, Locations, Performances (artist↔event with billing order)

## Reachability Risk
- **HIGH.** Three independent signals:
  1. API key program closed to new applicants since ~2017 ("not approving requests for student / educational / hobbyist purposes; standard partnership terms and license fee required"). Application form suspended. Last official comment Feb 2020 directed applicants to email Songkick's published support address (see songkick.com/contact).
  2. Suno acquisition Nov 2025 — strategy and API continuity now under an AI-music company, not the original WMG team that built Songkick.
  3. Public REST API itself responds (HTTP 401 with placeholder key, confirming reachability) but with no published rate-limit number and a 50-row max page size that forces page-stitching.
- Mitigation: Build BYO-key-first so the CLI is useful for anyone with a legacy/partner key. Public website (`www.songkick.com`) is `standard_http` reachable with no clearance cookie — opens room for a scrape-fallback later.

## Source Priority
- Primary: **api.songkick.com/api/3.0** (documented REST surface). Auth: paid/closed. Spec state: docs only, no OpenAPI — will author internal YAML from documented surface.
- No combo CLI; single-source build.

## Top Workflows
1. **Routing intelligence** — given an artist + target date/city (Jakarta default for Double Deer), find their nearest scheduled show with venue lat/lng, compute distance + days delta. Direct feed into the `tour-router` agent's `ArtistTourRouting` bundle.
2. **Artist gigography** — historical shows for an artist (proves a market presence in a region).
3. **Venue calendar** — what's coming to a specific venue (e.g., scout an Indonesian venue's current bookings).
4. **Metro area calendar** — what's coming to Jakarta (or any city) — feeds market-density analysis.
5. **Discovery search** — fuzzy search artists/venues/locations to resolve a name to a canonical Songkick ID.

## Data Layer
- Primary entities: `events`, `artists`, `venues`, `metro_areas`, `locations`, `performances` (artist↔event join with billing order)
- Sync cursor: cache responses keyed by `(endpoint, params)` with TTL; for an artist's calendar, sync `min_date` going forward.
- FTS/search: full-text index on artist `displayName`, venue `displayName`, location `city` for offline disambiguation. Pre-populate from sync results.
- Geo: store `venue.lat/lng` and `location.lat/lng` so distance computations work offline (great-circle from Jakarta = `-6.2088, 106.8456`).

## Codebase Intelligence
- No DeepWiki entry needed — wrapper surface area is small and well-documented at songkick.com/developer.
- Auth: query param `?apikey=<key>`. Env var convention to use: `SONGKICK_API_KEY`. The 401 response shape: `{"resultsPage": {"status": "error", "error": {"message": "..."}}}` — consistent error envelope.
- Rate limiting: no published number. Wrappers report no observed throttling at modest QPS; add a polite default limiter (~5 req/s) and surface a typed `RateLimitError` on 429.
- Data model: All responses wrap in `resultsPage` envelope with `status`, `results`, `totalEntries`, `page`, `perPage`. Always parse `resultsPage.results.{entity}[]`.

## Product Thesis
- Name: `songkick-pp-cli` (binary), Songkick CLI
- Headline: "The Songkick CLI for tour-routing and venue intelligence — BYO-key, offline-cached, agent-native output."
- Why it should exist: There is no Songkick CLI today. There is no Songkick MCP server today. The only wrappers (songkick-api-node, python-songkick) are abandoned JS/Python SDKs from 2014-2022. Every existing tool stops at "wrap the HTTP call"; nobody ships a local SQLite store, FTS search, routing-distance computation, agent-native JSON+select+csv output, or MCP exposure. Given the Suno acquisition and the closed key program, the durable angle is **routing intelligence over multiple sources** — this CLI is the Songkick leg of that stack (Bandsintown CLI being a separate build).
- Why Double Deer specifically: feeds the `tour-router` agent's `ArtistTourRouting` shape directly. Replaces ad-hoc WebFetch calls with a typed, cached, scriptable binary.

## Build Priorities
1. **Data layer + sync** for events, artists, venues, metro_areas, performances. Persist artist calendars; FTS-index names.
2. **Absorbed REST surface** — every documented endpoint in api.songkick.com/api/3.0: search/{artists,venues,locations,events}, events/{id}, events/{id}/setlists, artists/{id}/{calendar,gigography}, artists/mbid:{mbid}/..., venues/{id}, venues/{id}/calendar, metro_areas/{id}/calendar, users/{username}/{calendar,trackings,events,artists}. JSON only (skip XML — no value-add). Page-stitching past the 50-row cap built-in.
3. **Transcendence** — see absorb manifest. Headline novel features:
   - `route <artist> --from "Jakarta" --on 2026-09-15 --window 14` — routing-feasibility check using stored venue lat/lng + great-circle distance, returns daysFromTarget + distanceKm, exit code 0=strong-fit / 3=considered / 4=cold (matches `tour-router` verdict shape).
   - `gigography-density` — historical performance density per market for an artist (proves SEA familiarity).
   - `venue compare` — capacity + booking pace comparison across multiple venues.
4. **MCP exposure** — Cobratree mirror so the `tour-router` agent can call the CLI tools directly via MCP rather than shelling out.
5. **Polish** — terse-flag enrichment, README cookbook with routing examples, SKILL.md for agent discoverability.

## Auth Profile
- **API key** auth. `SONGKICK_API_KEY` env var. Query-param style (`?apikey=...`). No OAuth, no browser-session auth, no composed handshake.
- Live smoke testing skipped this run — user has no key (key program closed). CLI will be verified against mock responses and the documented surface only.

## Sources
- https://www.songkick.com/developer
- https://www.songkick.com/developer/getting-started
- https://www.songkick.com/developer/response-objects
- https://support.songkick.com/hc/en-us/articles/360012423194-Access-the-Songkick-API
- https://groups.google.com/g/songkick-api/c/EgWKrKtVis4
- https://github.com/schnogz/songkick-api-node
- https://github.com/mattdennewitz/python-songkick
- https://github.com/Integuru-AI/Songkick-Unofficial-API
- https://www.musicbusinessworldwide.com/on-suno-songkick-and-a-reddit-revolt/
