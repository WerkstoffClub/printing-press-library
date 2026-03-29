# espn-pp-cli

ESPN from your terminal. Live scores, standings, stats, and more across all major sports.

No auth required.

## Install

```bash
go install github.com/user/espn-pp-cli/cmd/espn-pp-cli@latest
```

## Quick start

```bash
espn-pp-cli scores nba
espn-pp-cli standings nfl
espn-pp-cli athlete "LeBron James" --league nba
espn-pp-cli team dal --league nfl --roster
espn-pp-cli search "Patrick Mahomes"
espn-pp-cli news nfl
espn-pp-cli boxscore 401671793 --league nfl
espn-pp-cli compare "LeBron James" "Kevin Durant" --league nba
espn-pp-cli dashboard
espn-pp-cli watch add Lakers --league nba
espn-pp-cli sync
```

## Agent and CI usage

Use `--agent` for machine-friendly defaults:

- Enables `--json`
- Enables `--compact`
- Enables `--no-input`
- Enables `--no-color`
- Enables `--yes`

Examples:

```bash
espn-pp-cli scores nba --agent
espn-pp-cli dashboard --agent
espn-pp-cli sync --agent
```

## Local data

Use `sync` to cache teams, athletes, events, standings, and news in a local SQLite database.

Use `search` to query live ESPN data and the local sync store together.

Use synced data for faster lookups and offline access to previously cached teams, athletes, standings, news, and watchlist workflows.

```bash
espn-pp-cli sync
espn-pp-cli sync --resources nfl,nba
espn-pp-cli search "Lakers"
```

Default database path:

```text
~/.local/share/espn-pp-cli/data.db
```

## Supported leagues

`NFL`, `NBA`, `MLB`, `NHL`, `NCAAF`, `NCAAM`, `NCAAW`, `MLS`, `EPL`, `WNBA`

## Note

ESPN APIs are undocumented and may change without notice.
