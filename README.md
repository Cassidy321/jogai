# jogai

Turn your AI coding sessions into daily dev logs.

jogai parses your [Claude Code](https://claude.com/product/claude-code) sessions, generates a summary using Claude, and saves it as a markdown file — ready for [Obsidian](https://obsidian.md) or any note-taking tool.

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
# 1. Set up jogai (choose output folder + day boundary)
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

## Usage

### Generate a recap

```bash
# Recap the previous (completed) dev day
jogai run

# Recap a specific dev day
jogai run --day 2026-04-01
```

All recaps are written as `YYYY-MM-DD.md`. Manual and scheduled runs produce the same file for the same day — the overwrite is idempotent.

### Schedule daily recaps

```bash
# Install a launchd job that fires at your day_end
jogai schedule start

# Check status
jogai schedule status

# Stop
jogai schedule stop
```

The schedule runs `jogai run` automatically at the configured `day_end`. If your Mac is asleep, macOS Power Nap wakes it briefly to run the job. If it's fully off, launchd catches up when it wakes.

To change the schedule time, re-run `jogai init` to update `day_end`, then `jogai schedule stop && jogai schedule start` to regenerate the plist.

### Check system health

```bash
jogai status
```

```
jogai status

  Parser:     ✓ Claude Code
  Summarizer: ✓ claude CLI
  Output:     /Users/you/jogai-recaps
  Schedule:   daily at 05:00, next run 2026-04-20 05:00
```

If a scheduled run didn't produce a file (e.g. permission denied, Mac off), `jogai status` displays the exact catch-up command.

## Requirements

- [Claude Code](https://claude.com/product/claude-code) installed and authenticated
- macOS for scheduling (Linux/Windows coming soon)

## How It Works

1. **Parse** — reads Claude Code session history from `~/.claude/projects/`
2. **Filter** — collapses code blocks and truncates long messages to reduce tokens
3. **Summarize** — sends filtered sessions to Claude for a concise recap
4. **Write** — saves the recap as `YYYY-MM-DD.md` in your output directory

## License

[MIT](LICENSE)
