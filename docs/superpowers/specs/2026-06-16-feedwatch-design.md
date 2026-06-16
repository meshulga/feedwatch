# feedwatch — Design Spec

**Date:** 2026-06-16

## Overview

feedwatch is a Go-based Telegram user-client that monitors configurable source chats, filters messages by keyword rules, and forwards matching posts to designated output channels. Managed entirely via Telegram commands — no config files to edit manually.

---

## Architecture

Single Go process using gotd/td (MTProto user-client). Internally composed of four layers:

```
Telegram (sources) → client → pipeline → Telegram (outputs)
```

- **client** — connects via MTProto as a user account, receives update stream
- **pipeline** — routes each message through filter, forwards matches to output chat
- **filter** — evaluates stop-words and run-words against message text
- **bot** — listens for commands from the owner, mutates config at runtime

Config is held in memory and persisted to `config.json` on every change. On restart, config is loaded from disk; no deduplication (SQLite dedup is a future addition).

---

## Components

### `cmd/feedwatch/main.go`
Entry point. Loads config, initializes gotd/td session, starts bot command handler and update listener.

### `internal/client`
Wraps gotd/td. Authenticates user session (phone → code → 2FA if needed). Exposes a callback for incoming messages. Handles reconnects.

### `internal/pipeline`
Core data model:

```go
type Pipeline struct {
    ID      string
    Sources []int64   // chat IDs to watch
    Filter  Filter
    Output  int64     // destination chat ID
}
```

Multiple pipelines are supported from day one. Each message is evaluated against all pipelines whose sources include the originating chat.

### `internal/filter`
```go
type Filter struct {
    RunWords  []string  // message must contain at least one
    StopWords []string  // message must contain none
}
```

Case-insensitive substring match. A message passes if:
- it contains at least one run-word (or run-words list is empty)
- it contains no stop-words

### `internal/config`
Loads/saves `config.json`. Exposes `Config` struct with list of pipelines and owner Telegram user ID. Thread-safe writes via mutex.

### `internal/bot`
Handles Telegram commands sent by the owner user:

| Command | Action |
|---------|--------|
| `/new_pipeline <name>` | Create pipeline |
| `/add_source <pipeline> <chat_id>` | Add source chat |
| `/set_output <pipeline> <chat_id>` | Set output channel |
| `/add_run <pipeline> <word>` | Add run-word |
| `/add_stop <pipeline> <word>` | Add stop-word |
| `/list` | Show all pipelines with config |
| `/del_pipeline <pipeline>` | Delete pipeline |

Commands are only accepted from the configured owner user ID.

---

## Data Flow

1. gotd/td receives `UpdateNewMessage`
2. client extracts chat ID + message text, passes to pipeline router
3. pipeline router finds all pipelines where `Sources` contains the chat ID
4. filter evaluates run-words and stop-words
5. on match: client sends message to `Output` chat

---

## Config Format (`config.json`)

```json
{
  "owner_id": 123456789,
  "pipelines": [
    {
      "id": "go-jobs",
      "sources": [-1001234567890],
      "output": -1009876543210,
      "filter": {
        "run_words": ["golang", "go developer", "remote"],
        "stop_words": ["junior", "стажёр", "intern"]
      }
    }
  ]
}
```

---

## Error Handling

- Auth errors on startup → log and exit (user must re-auth)
- Failed message forward → log warning, continue (non-fatal)
- Config save failure → log error, keep running with in-memory state
- Unknown commands → reply with usage hint

---

## Out of Scope (v1)

- SQLite deduplication (add later)
- LLM-based filtering
- Web UI
- Docker / VPS deploy
- Regex patterns in filters

---

## Tech Stack

| Component | Choice |
|-----------|--------|
| Language | Go 1.22+ |
| Telegram | gotd/td |
| Config | encoding/json |
| Persistence | JSON file (SQLite later) |
| Deploy | single binary, run locally |
