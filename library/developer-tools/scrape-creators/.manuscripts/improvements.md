# PR #113 Quality Improvements

Four proposed improvements to land before or alongside PR #113. Each is additive; none change API behavior or break existing invocations.

## 1. Platform action command naming

See `action-map-proposal.md` in this directory. Renames the 82 operation-ID leaf names (`list`, `list-post-2`, `list-user-5`, `list-adlibrary-3`) to `platform action` pairs matching v1's `@scrapecreators/cli` convention (`tiktok profile`, `instagram user-posts`, `facebook adlibrary-search-ads`). Splits the dual-role `promoted_<platform>.go` files so per-platform cobra parents are pure navigation. Operation-ID names and former top-level parent shortcuts become hidden cobra aliases for backward compatibility.

Blocks on: Adrian reviewing the 115-endpoint map.

## 2. Interactive wizard on bare invocation

Bare `scrape-creators-pp-cli` in a TTY launches a guided wizard mirroring v1's `@clack/prompts` three-step flow (platform, action, required params). Non-TTY invocations and anything with `--no-input`, `--agent`, or `--yes` keep printing help. Library: `github.com/charmbracelet/huh` unless integration measurement crosses 5 MB over baseline, in which case fall back to `survey/v2`.

## 3. `agent add` auto-wiring

`scrape-creators-pp-cli agent add cursor|claude-desktop|claude-code|codex` writes a valid MCP server entry into the target's config with `0600` permissions. `--hosted` writes the `api.scrapecreators.com/mcp` URL; default writes the local `scrape-creators-pp-mcp` stdio binary. Cursor, Claude Desktop, and Claude Code share a JSON helper (`mcpServers` shape); Codex uses a TOML helper via `pelletier/go-toml/v2`. Claude Code writes directly to `~/.claude.json`; no `claude mcp add` shell-out, so there is no PATH dependency.

Refuse to overwrite an existing `scrape-creators-pp-cli` or `scrapecreators` entry without `--force`; print a diff.

## 4. Client-side input normalization

New `internal/cli/input_normalize.go` with `NormalizeHandle` (strip one leading `@`, trim whitespace) and `NormalizeHashtag` (strip one leading `#`, trim whitespace). Applied in every leaf that accepts a handle or hashtag parameter (~30 files). README surfaces the rules in a dedicated block up front so the library documents its own behavior, not upstream API tolerance.

## Ordering

1. Review and agree on the action map (this PR). Blocks on Adrian.
2. After PR #113 merges, land the cobra restructure (#1) on top.
3. Improvements #2, #3, #4 can land in parallel with #1 since they touch new files or additive sections of `root.go` only.

## Out of scope here

- Binary rename from `scrape-creators-pp-cli` to `scrapecreators`.
- npm distribution via `@scrapecreators/cli`.
- `curl | sh` installer.
- Retiring v1 or transferring repo home.

Those are adoption / handoff work Adrian owns when he is ready to port this into `@scrapecreators/cli`. Tracked separately.

## Companion plan

Full implementation plan with file lists, test scenarios, and risk mitigations: `docs/plans/2026-04-23-002-feat-pr-113-library-quality-plan.md` on `main`.
