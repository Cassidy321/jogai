# jogai

Turn your AI coding sessions into daily dev logs.

jogai parses your [Claude Code](https://claude.com/product/claude-code) and [Codex](https://github.com/openai/codex) sessions, generates a summary using Claude or Codex, and saves it as a markdown file — ready for [Obsidian](https://obsidian.md) or any note-taking tool.

## Installation

### Homebrew (macOS / Linux)

```bash
brew install cassidy321/tap/jogai
```

### From source

```bash
go install github.com/Cassidy321/jogai/cmd/jogai@latest
```

## Quick Start

```bash
# 1. Set up jogai (pick your sources, summarizer, output folder + day boundary)
jogai init

# 2. Generate your first recap
jogai run
```

A markdown file appears in your output directory.

## Core concept: the dev day

jogai summarizes a **dev day** — a 24-hour window anchored on an hour you choose.

- `day_end = 00:00` (default) → calendar days (midnight to midnight)
- `day_end = 05:00` → dev day runs 05:00 to 05:00 the next morning, so late-night sessions stay on the day you worked them

Every recap targets a specific dev day. The file name is the **date the window starts**, so `2026-04-17.md` = "what you did on April 17" regardless of your boundary.

A recap has one section per project — the git repository (or folder) the session ran in — busiest first, and a last `## Hors projet` section for one-off questions asked outside any project:

```markdown
# 2026-04-17

## dokaa

- Embedded ticket form: iframe + optional embed.js …

## jogai

- …

## Hors projet

- …
```

If one project fails to summarize, the others are still written and the day is retried on the next run. A recap you edited by hand is never replaced by a catch-up; `jogai run --day 2026-04-17 --force` replaces it on purpose.

## Usage

### Generate a recap

```bash
# Recap every completed dev day that has no recap yet (up to 14 days back)
jogai run

# Recap, or regenerate, a specific dev day
jogai run --day 2026-04-01
```

All recaps are written as `YYYY-MM-DD.md`. jogai remembers the outcome of each dev day: a day that failed (network, sleep, rate limit) is retried automatically on the next run, and a day without sessions is not tried again.

### Schedule daily recaps

```bash
jogai schedule start    # install the launchd job
jogai schedule status
jogai schedule stop
```

The job runs `jogai run` at your `day_end` and each time you log in. If your Mac was asleep at `day_end`, launchd runs it when the Mac wakes; if it was off, the run at login catches up. Every dev day missed in between is recapped, up to 14 days back.

Changing `day_end` with `jogai init` updates the schedule automatically, and so does upgrading jogai.

### Updates

When jogai is installed with Homebrew, the scheduled run upgrades it at most once a day (`brew upgrade cassidy321/tap/jogai`). To turn this off, add `"auto_update": false` to `~/.config/jogai/config.json`.

### Session archive

Claude Code deletes transcripts after about 30 days. Every run, jogai copies what is new in your interactive Claude Code and Codex sessions into a private archive (`~/.local/share/jogai/jogai.db`), so your history outlives that cleanup. Credentials (API keys, tokens, passwords in assignments or URLs) are masked before anything is stored, and automated sessions (SDK, `claude -p`, `codex exec`) are left out.

### Search your past sessions

```bash
jogai search caffeinate launchd
jogai search --project dokaa --since 2026-09-01 embed iframe
jogai search --recaps retry
```

`jogai init` (and the first run after an upgrade) also registers jogai as an MCP server in Claude Code, for all your projects. Claude then searches your archive by itself when you mention past work — "how did we fix the TCC prompts?" — and reads the whole exchange around what it finds. If something needs your attention (a recap failing for days, an expired login, a failed update), Claude tells you at the start of a session.

### Uninstall

```bash
jogai uninstall          # schedule + Claude Code integration
jogai uninstall --data   # also the archive and settings
brew uninstall jogai
```

### Check system health

```bash
jogai status
```

```
jogai status

  Sources:    ✓ Claude Code
              ✓ Codex
  Summarizer: ✓ claude CLI
  Output:     /Users/you/jogai-recaps
  Archive:    12456 messages from 152 sessions, updated 2026-04-20 05:00
  Schedule:   daily at 05:00, next run 2026-04-20 05:00
  Last run:   2026-04-19 05:00 (dev day 2026-04-18) — ok
```

`jogai status` lists the recent dev days that failed (retried on the next run) or were refused by the model (with the command to retry them by hand), and how many days are still waiting for the next run. If a source failed but others succeeded, the recap is still written and a warning blockquote is added above the body.

## Requirements

- At least one of [Claude Code](https://claude.com/product/claude-code) or the [Codex CLI](https://github.com/openai/codex) installed and authenticated — either tool can act as a session source, a summarizer, or both. Install both to fuse Claude Code and Codex sessions into a single daily recap.
- macOS for scheduling

## How It Works

1. **Archive** — copies what is new in the sessions you picked at `jogai init` (Claude Code from `~/.claude/projects/`, Codex from `~/.codex/sessions/`) into the private archive, leaving out automated sessions and masking credentials
2. **Filter** — keeps what you typed and what the assistant answered; drops injected context and command output; collapses code blocks and fits each project into a bounded prompt
3. **Summarize** — one call per project to your chosen summarizer (Claude or Codex)
4. **Write** — assembles the sections into `YYYY-MM-DD.md` in your output directory

## License

[MIT](LICENSE)
