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
# 1. Set up jogai (choose where to save recaps)
jogai init

# 2. Generate your first recap
jogai run
```

That's it. A markdown file appears in your output directory.

## Usage

### Generate a recap

```bash
# Recap of the last 24 hours
jogai run

# Recap of a specific day
jogai run --day 2026-04-01
```

### Schedule daily recaps

```bash
# Run automatically every day at 9am
jogai schedule start --at 09:00

# The scheduled run summarizes the fixed window
# from yesterday 09:00 to today 09:00

# Check schedule status
jogai schedule status

# Stop the schedule
jogai schedule stop
```

### Check system health

```bash
jogai status
```

```
jogai status

  Parser:     ✓ Claude Code
  Summarizer: ✓ claude CLI
  Output:     /Users/you/jogai-recaps
```

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
