# feedwatch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build feedwatch — a Go user-client that monitors Telegram source chats, filters messages by keyword rules, and forwards matching posts to output channels, managed entirely via Telegram commands.

**Architecture:** Single Go binary using gotd/td (MTProto user client). Messages flow through a pipeline router: source chats → filter evaluation → forward to output channel. Config persists to `config.json`; a bot handler listens for owner commands and mutates config at runtime.

**Tech Stack:** Go 1.22+, `github.com/gotd/td` (MTProto), `encoding/json`, standard library.

---

## File Structure

| File | Responsibility |
|------|----------------|
| `go.mod` / `go.sum` | Module definition, dependencies |
| `cmd/feedwatch/main.go` | Entry point, wires all packages |
| `internal/filter/filter.go` | `Filter` struct + `Matches()` |
| `internal/filter/filter_test.go` | Unit tests for filter logic |
| `internal/pipeline/pipeline.go` | `Peer`, `Pipeline` types + `Router` |
| `internal/pipeline/pipeline_test.go` | Unit tests for routing logic |
| `internal/config/config.go` | `Config` struct + `Load`/`Save` + mutation methods |
| `internal/config/config_test.go` | Roundtrip persistence tests |
| `internal/bot/bot.go` | `ParseCommand` + `Handler` |
| `internal/bot/bot_test.go` | Command parsing unit tests |
| `internal/client/client.go` | gotd/td wrapper: auth, receive, send, forward, resolve |

---

## Task 1: Initialize Go module

**Files:**
- Create: `go.mod`
- Create: directory tree

- [ ] **Step 1: Create directory structure**

```bash
cd /path/to/feedwatch
mkdir -p cmd/feedwatch internal/filter internal/pipeline internal/config internal/bot internal/client
```

- [ ] **Step 2: Initialize Go module**

```bash
go mod init feedwatch
```

Expected `go.mod`:
```
module feedwatch

go 1.22
```

- [ ] **Step 3: Add dependencies**

```bash
go get github.com/gotd/td@latest
```

- [ ] **Step 4: Verify module is valid**

```bash
go mod tidy
```

Expected: no errors, `go.sum` created.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: initialize Go module with gotd/td"
```

---

## Task 2: Filter package

**Files:**
- Create: `internal/filter/filter_test.go`
- Create: `internal/filter/filter.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/filter/filter_test.go`:

```go
package filter_test

import (
	"testing"

	"feedwatch/internal/filter"
)

func TestMatches_RunWords(t *testing.T) {
	f := filter.Filter{RunWords: []string{"golang", "go developer"}}

	if !f.Matches("We need a Go Developer for a remote role") {
		t.Error("expected match on 'go developer' (case-insensitive)")
	}
	if !f.Matches("Looking for a golang engineer") {
		t.Error("expected match on 'golang'")
	}
	if f.Matches("We need a Python developer") {
		t.Error("expected no match")
	}
}

func TestMatches_StopWords(t *testing.T) {
	f := filter.Filter{RunWords: []string{"golang"}, StopWords: []string{"junior", "стажёр"}}

	if f.Matches("Junior golang developer wanted") {
		t.Error("expected stop on 'junior'")
	}
	if f.Matches("Golang стажёр position") {
		t.Error("expected stop on 'стажёр'")
	}
	if !f.Matches("Senior golang developer wanted") {
		t.Error("expected match: has run-word, no stop-words")
	}
}

func TestMatches_EmptyRunWords(t *testing.T) {
	f := filter.Filter{StopWords: []string{"spam"}}

	if !f.Matches("Any message here") {
		t.Error("expected match: no run-words required")
	}
	if f.Matches("This is spam content") {
		t.Error("expected stop on 'spam'")
	}
}

func TestMatches_EmptyFilter(t *testing.T) {
	f := filter.Filter{}
	if !f.Matches("anything at all") {
		t.Error("empty filter should match everything")
	}
}

func TestMatches_StopWordBeforeRunWord(t *testing.T) {
	f := filter.Filter{RunWords: []string{"golang"}, StopWords: []string{"junior"}}
	// stop-word check happens first
	if f.Matches("junior golang position") {
		t.Error("stop-word should block even when run-word present")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/filter/...
```

Expected: `cannot find package "feedwatch/internal/filter"`

- [ ] **Step 3: Implement filter.go**

Create `internal/filter/filter.go`:

```go
package filter

import "strings"

type Filter struct {
	RunWords  []string `json:"run_words"`
	StopWords []string `json:"stop_words"`
}

func (f Filter) Matches(text string) bool {
	lower := strings.ToLower(text)
	for _, sw := range f.StopWords {
		if strings.Contains(lower, strings.ToLower(sw)) {
			return false
		}
	}
	if len(f.RunWords) == 0 {
		return true
	}
	for _, rw := range f.RunWords {
		if strings.Contains(lower, strings.ToLower(rw)) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/filter/... -v
```

Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/filter/
git commit -m "feat: add filter package with run-word/stop-word matching"
```

---

## Task 3: Pipeline package

**Files:**
- Create: `internal/pipeline/pipeline_test.go`
- Create: `internal/pipeline/pipeline.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/pipeline/pipeline_test.go`:

```go
package pipeline_test

import (
	"testing"

	"feedwatch/internal/filter"
	"feedwatch/internal/pipeline"
)

func TestRouter_Route_Match(t *testing.T) {
	router := pipeline.NewRouter([]pipeline.Pipeline{
		{
			ID:      "go-jobs",
			Sources: []pipeline.Peer{{ID: 1001, Type: "channel"}},
			Filter:  filter.Filter{RunWords: []string{"golang"}},
			Output:  &pipeline.Peer{ID: 2001, AccessHash: 9999, Type: "channel"},
		},
	})

	matched := router.Route(1001, "Senior golang engineer needed")
	if len(matched) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matched))
	}
	if matched[0].ID != "go-jobs" {
		t.Errorf("expected pipeline 'go-jobs', got %q", matched[0].ID)
	}
}

func TestRouter_Route_NoRunWord(t *testing.T) {
	router := pipeline.NewRouter([]pipeline.Pipeline{
		{
			ID:      "p1",
			Sources: []pipeline.Peer{{ID: 1001, Type: "channel"}},
			Filter:  filter.Filter{RunWords: []string{"golang"}},
			Output:  &pipeline.Peer{ID: 2001, Type: "channel"},
		},
	})

	matched := router.Route(1001, "Python developer position")
	if len(matched) != 0 {
		t.Errorf("expected no match, got %d", len(matched))
	}
}

func TestRouter_Route_WrongSource(t *testing.T) {
	router := pipeline.NewRouter([]pipeline.Pipeline{
		{
			ID:      "p1",
			Sources: []pipeline.Peer{{ID: 1001, Type: "channel"}},
			Filter:  filter.Filter{RunWords: []string{"golang"}},
			Output:  &pipeline.Peer{ID: 2001, Type: "channel"},
		},
	})

	matched := router.Route(9999, "golang developer needed")
	if len(matched) != 0 {
		t.Errorf("expected no match for unknown source, got %d", len(matched))
	}
}

func TestRouter_Route_MultiPipeline(t *testing.T) {
	router := pipeline.NewRouter([]pipeline.Pipeline{
		{
			ID:      "p1",
			Sources: []pipeline.Peer{{ID: 1001, Type: "channel"}},
			Filter:  filter.Filter{RunWords: []string{"golang"}},
			Output:  &pipeline.Peer{ID: 2001, Type: "channel"},
		},
		{
			ID:      "p2",
			Sources: []pipeline.Peer{{ID: 1001, Type: "channel"}},
			Filter:  filter.Filter{RunWords: []string{"remote"}},
			Output:  &pipeline.Peer{ID: 2002, Type: "channel"},
		},
	})

	// Both pipelines watch same source; message matches both
	matched := router.Route(1001, "golang remote position")
	if len(matched) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matched))
	}
}

func TestRouter_Route_StopWord(t *testing.T) {
	router := pipeline.NewRouter([]pipeline.Pipeline{
		{
			ID:      "p1",
			Sources: []pipeline.Peer{{ID: 1001, Type: "channel"}},
			Filter:  filter.Filter{RunWords: []string{"golang"}, StopWords: []string{"junior"}},
			Output:  &pipeline.Peer{ID: 2001, Type: "channel"},
		},
	})

	matched := router.Route(1001, "junior golang developer")
	if len(matched) != 0 {
		t.Errorf("expected no match due to stop-word, got %d", len(matched))
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/pipeline/...
```

Expected: `cannot find package "feedwatch/internal/pipeline"`

- [ ] **Step 3: Implement pipeline.go**

Create `internal/pipeline/pipeline.go`:

```go
package pipeline

import "feedwatch/internal/filter"

// Peer identifies a Telegram chat with its MTProto access credentials.
// ID is the raw MTProto ID: positive for channels and users, negative for groups.
// Type is "channel", "chat", or "user".
type Peer struct {
	ID         int64  `json:"id"`
	AccessHash int64  `json:"access_hash,omitempty"`
	Type       string `json:"type,omitempty"`
}

type Pipeline struct {
	ID      string        `json:"id"`
	Sources []Peer        `json:"sources"`
	Filter  filter.Filter `json:"filter"`
	Output  *Peer         `json:"output,omitempty"`
}

type Router struct {
	pipelines []Pipeline
}

func NewRouter(pipelines []Pipeline) *Router {
	ps := make([]Pipeline, len(pipelines))
	copy(ps, pipelines)
	return &Router{pipelines: ps}
}

// Route returns all pipelines that match the given source chat and message text.
func (r *Router) Route(chatID int64, text string) []Pipeline {
	var matched []Pipeline
	for _, p := range r.pipelines {
		for _, src := range p.Sources {
			if src.ID == chatID {
				if p.Filter.Matches(text) {
					matched = append(matched, p)
				}
				break
			}
		}
	}
	return matched
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/pipeline/... -v
```

Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/
git commit -m "feat: add pipeline package with Peer types and Router"
```

---

## Task 4: Config package

**Files:**
- Create: `internal/config/config_test.go`
- Create: `internal/config/config.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/config/config_test.go`:

```go
package config_test

import (
	"path/filepath"
	"testing"

	"feedwatch/internal/config"
	"feedwatch/internal/filter"
	"feedwatch/internal/pipeline"
)

func TestLoad_NotExists(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OwnerID() != 0 {
		t.Errorf("expected owner_id=0, got %d", cfg.OwnerID())
	}
	if len(cfg.Pipelines()) != 0 {
		t.Errorf("expected empty pipelines, got %d", len(cfg.Pipelines()))
	}
}

func TestSaveLoad_Roundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	cfg.SetOwnerID(123456789)
	cfg.AddPipeline(pipeline.Pipeline{
		ID:      "go-jobs",
		Sources: []pipeline.Peer{{ID: 1001, AccessHash: 5555, Type: "channel"}},
		Filter:  filter.Filter{RunWords: []string{"golang"}, StopWords: []string{"junior"}},
		Output:  &pipeline.Peer{ID: 2001, AccessHash: 6666, Type: "channel"},
	})

	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	cfg2, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg2.OwnerID() != 123456789 {
		t.Errorf("owner_id: got %d, want 123456789", cfg2.OwnerID())
	}
	ps := cfg2.Pipelines()
	if len(ps) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(ps))
	}
	p := ps[0]
	if p.ID != "go-jobs" {
		t.Errorf("pipeline ID: got %q, want 'go-jobs'", p.ID)
	}
	if len(p.Sources) != 1 || p.Sources[0].ID != 1001 {
		t.Errorf("sources: %v", p.Sources)
	}
	if p.Output == nil || p.Output.ID != 2001 {
		t.Errorf("output: %v", p.Output)
	}
	if len(p.Filter.RunWords) != 1 || p.Filter.RunWords[0] != "golang" {
		t.Errorf("run_words: %v", p.Filter.RunWords)
	}
}

func TestUpdatePipeline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, _ := config.Load(path)
	cfg.AddPipeline(pipeline.Pipeline{ID: "p1"})

	updated := pipeline.Pipeline{ID: "p1", Filter: filter.Filter{RunWords: []string{"go"}}}
	if !cfg.UpdatePipeline(updated) {
		t.Fatal("UpdatePipeline returned false")
	}

	p, ok := cfg.FindPipeline("p1")
	if !ok {
		t.Fatal("pipeline not found after update")
	}
	if len(p.Filter.RunWords) != 1 || p.Filter.RunWords[0] != "go" {
		t.Errorf("run_words not updated: %v", p.Filter.RunWords)
	}
}

func TestDeletePipeline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, _ := config.Load(path)
	cfg.AddPipeline(pipeline.Pipeline{ID: "p1"})
	cfg.AddPipeline(pipeline.Pipeline{ID: "p2"})

	if !cfg.DeletePipeline("p1") {
		t.Fatal("DeletePipeline returned false")
	}
	if len(cfg.Pipelines()) != 1 {
		t.Errorf("expected 1 pipeline after delete, got %d", len(cfg.Pipelines()))
	}
	if cfg.DeletePipeline("nonexistent") {
		t.Error("DeletePipeline should return false for unknown ID")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/config/...
```

Expected: `cannot find package "feedwatch/internal/config"`

- [ ] **Step 3: Implement config.go**

Create `internal/config/config.go`:

```go
package config

import (
	"encoding/json"
	"os"
	"sync"

	"feedwatch/internal/pipeline"
)

type configData struct {
	OwnerID   int64               `json:"owner_id"`
	Pipelines []pipeline.Pipeline `json:"pipelines"`
}

type Config struct {
	mu   sync.RWMutex
	data configData
	path string
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{path: path}, nil
	}
	if err != nil {
		return nil, err
	}
	var d configData
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	return &Config{data: d, path: path}, nil
}

func (c *Config) Save() error {
	c.mu.RLock()
	d := c.data
	c.mu.RUnlock()

	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, b, 0600)
}

func (c *Config) OwnerID() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data.OwnerID
}

func (c *Config) SetOwnerID(id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data.OwnerID = id
}

func (c *Config) Pipelines() []pipeline.Pipeline {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]pipeline.Pipeline, len(c.data.Pipelines))
	copy(result, c.data.Pipelines)
	return result
}

func (c *Config) AddPipeline(p pipeline.Pipeline) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data.Pipelines = append(c.data.Pipelines, p)
}

func (c *Config) FindPipeline(id string) (pipeline.Pipeline, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, p := range c.data.Pipelines {
		if p.ID == id {
			return p, true
		}
	}
	return pipeline.Pipeline{}, false
}

func (c *Config) UpdatePipeline(updated pipeline.Pipeline) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, p := range c.data.Pipelines {
		if p.ID == updated.ID {
			c.data.Pipelines[i] = updated
			return true
		}
	}
	return false
}

func (c *Config) DeletePipeline(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, p := range c.data.Pipelines {
		if p.ID == id {
			c.data.Pipelines = append(c.data.Pipelines[:i], c.data.Pipelines[i+1:]...)
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/config/... -v
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config package with thread-safe load/save and pipeline mutations"
```

---

## Task 5: Bot command parsing

**Files:**
- Create: `internal/bot/bot_test.go`
- Create: `internal/bot/bot.go` (ParseCommand only)

- [ ] **Step 1: Write the failing tests**

Create `internal/bot/bot_test.go`:

```go
package bot_test

import (
	"testing"

	"feedwatch/internal/bot"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input string
		name  string
		args  []string
		ok    bool
	}{
		{"/new_pipeline my-feed", "new_pipeline", []string{"my-feed"}, true},
		{"/add_source my-feed @golang_jobs", "add_source", []string{"my-feed", "@golang_jobs"}, true},
		{"/set_output my-feed @my_channel", "set_output", []string{"my-feed", "@my_channel"}, true},
		{"/add_run my-feed golang", "add_run", []string{"my-feed", "golang"}, true},
		{"/add_stop my-feed junior", "add_stop", []string{"my-feed", "junior"}, true},
		{"/del_pipeline my-feed", "del_pipeline", []string{"my-feed"}, true},
		{"/list", "list", []string{}, true},
		{"hello world", "", nil, false},
		{"", "", nil, false},
		{"/", "", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, args, ok := bot.ParseCommand(tt.input)
			if ok != tt.ok {
				t.Fatalf("ParseCommand(%q): ok=%v, want %v", tt.input, ok, tt.ok)
			}
			if !ok {
				return
			}
			if name != tt.name {
				t.Errorf("name=%q, want %q", name, tt.name)
			}
			if len(args) != len(tt.args) {
				t.Errorf("args=%v, want %v", args, tt.args)
				return
			}
			for i := range args {
				if args[i] != tt.args[i] {
					t.Errorf("args[%d]=%q, want %q", i, args[i], tt.args[i])
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/bot/...
```

Expected: `cannot find package "feedwatch/internal/bot"`

- [ ] **Step 3: Implement ParseCommand**

Create `internal/bot/bot.go`:

```go
package bot

import "strings"

// ParseCommand parses a Telegram command text (e.g. "/add_run my-feed golang").
// Returns command name (without /), args slice, and whether it is a command.
func ParseCommand(text string) (name string, args []string, ok bool) {
	if len(text) == 0 || text[0] != '/' {
		return "", nil, false
	}
	parts := strings.Fields(text)
	name = strings.TrimPrefix(parts[0], "/")
	if name == "" {
		return "", nil, false
	}
	return name, parts[1:], true
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/bot/... -v
```

Expected: all 10 cases PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bot/
git commit -m "feat: add bot package with ParseCommand"
```

---

## Task 6: Client package

**Files:**
- Create: `internal/client/client.go`

No unit tests (gotd/td integration requires a live session). Verify with `go build`.

- [ ] **Step 1: Create client.go**

Create `internal/client/client.go`:

```go
package client

import (
	"context"
	"encoding/binary"
	"crypto/rand"
	"fmt"
	"log"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/session"

	"feedwatch/internal/pipeline"
)

// MessageEvent is delivered for each relevant incoming message.
type MessageEvent struct {
	ChatID         int64
	ChatAccessHash int64
	ChatType       string // "channel", "chat", or "user"
	SenderID       int64
	MessageID      int
	Text           string
	// IsOwnerCommand is true when the sender is the configured owner,
	// including messages in Saved Messages (outgoing-to-self).
	IsOwnerCommand bool
}

// Handler is called for each message event.
type Handler func(ctx context.Context, event MessageEvent)

type Client struct {
	appID       int
	appHash     string
	sessionPath string
	ownerID     int64
	api         *tg.Client
}

func New(appID int, appHash string, sessionPath string, ownerID int64) *Client {
	return &Client{
		appID:       appID,
		appHash:     appHash,
		sessionPath: sessionPath,
		ownerID:     ownerID,
	}
}

// Run connects, authenticates (interactive terminal on first run), then calls
// handler for every incoming message until ctx is cancelled.
func (c *Client) Run(ctx context.Context, handler Handler) error {
	dispatcher := tg.NewUpdateDispatcher()

	tc := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: c.sessionPath},
		UpdateHandler:  dispatcher,
	})

	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		msg, ok := update.Message.(*tg.Message)
		if !ok {
			return nil
		}
		ev := c.buildEvent(msg, e)
		if ev == nil {
			return nil
		}
		handler(ctx, *ev)
		return nil
	})

	return tc.Run(ctx, func(ctx context.Context) error {
		c.api = tc.API()

		if err := auth.NewFlow(
			auth.Terminal{},
			auth.SendCodeOptions{},
		).Run(ctx, tc.Auth()); err != nil {
			return fmt.Errorf("auth: %w", err)
		}

		log.Println("feedwatch: authenticated, listening for messages")
		<-ctx.Done()
		return ctx.Err()
	})
}

// SendMessage sends a text message to the given chat.
// When chatID equals ownerID (Saved Messages), uses InputPeerSelf to avoid
// needing an access hash for the user's own account.
func (c *Client) SendMessage(ctx context.Context, chatID int64, accessHash int64, text string) error {
	var peer tg.InputPeerClass
	if chatID == c.ownerID {
		peer = &tg.InputPeerSelf{}
	} else {
		peer = idToPeer(chatID, accessHash)
	}
	_, err := c.api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:      peer,
		Message:   text,
		RandomID:  randomID(),
		NoWebpage: true,
	})
	return err
}

// ForwardMessage forwards a message from one peer to another.
func (c *Client) ForwardMessage(ctx context.Context, from pipeline.Peer, msgID int, to pipeline.Peer) error {
	_, err := c.api.MessagesForwardMessages(ctx, &tg.MessagesForwardMessagesRequest{
		FromPeer: peerToInput(from),
		ID:       []int{msgID},
		ToPeer:   peerToInput(to),
		RandomID: []int64{randomID()},
		Silent:   true,
	})
	return err
}

// ResolvePeer resolves a Telegram username (with or without @) to a Peer.
// The returned Peer contains the MTProto ID and access hash needed for API calls.
func (c *Client) ResolvePeer(ctx context.Context, username string) (pipeline.Peer, error) {
	if len(username) > 0 && username[0] == '@' {
		username = username[1:]
	}
	resolved, err := c.api.ContactsResolveUsername(ctx, username)
	if err != nil {
		return pipeline.Peer{}, fmt.Errorf("resolve %q: %w", username, err)
	}
	switch p := resolved.Peer.(type) {
	case *tg.PeerChannel:
		for _, chat := range resolved.Chats {
			if ch, ok := chat.(*tg.Channel); ok && ch.ID == p.ChannelID {
				return pipeline.Peer{ID: ch.ID, AccessHash: ch.AccessHash, Type: "channel"}, nil
			}
		}
	case *tg.PeerUser:
		for _, u := range resolved.Users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				return pipeline.Peer{ID: user.ID, AccessHash: user.AccessHash, Type: "user"}, nil
			}
		}
	case *tg.PeerChat:
		for _, chat := range resolved.Chats {
			if ch, ok := chat.(*tg.Chat); ok && ch.ID == p.ChatID {
				return pipeline.Peer{ID: -ch.ID, Type: "chat"}, nil
			}
		}
	}
	return pipeline.Peer{}, fmt.Errorf("peer not found in response for %q", username)
}

func (c *Client) buildEvent(msg *tg.Message, e tg.Entities) *MessageEvent {
	ev := &MessageEvent{
		MessageID: msg.ID,
		Text:      msg.Message,
	}

	switch p := msg.PeerID.(type) {
	case *tg.PeerChannel:
		ev.ChatID = p.ChannelID
		ev.ChatType = "channel"
		if ch, ok := e.Channels[p.ChannelID]; ok {
			ev.ChatAccessHash = ch.AccessHash
		}
	case *tg.PeerChat:
		ev.ChatID = -p.ChatID
		ev.ChatType = "chat"
	case *tg.PeerUser:
		ev.ChatID = p.UserID
		ev.ChatType = "user"
	}

	if msg.Out {
		// Only care about outgoing messages to Saved Messages (self-commands).
		if p, ok := msg.PeerID.(*tg.PeerUser); ok && p.UserID == c.ownerID {
			ev.SenderID = c.ownerID
			ev.IsOwnerCommand = true
		} else {
			return nil
		}
	} else {
		if p, ok := msg.FromID.(*tg.PeerUser); ok {
			ev.SenderID = p.UserID
			ev.IsOwnerCommand = (p.UserID == c.ownerID)
		}
	}

	return ev
}

// idToPeer converts a conventional chat ID and access hash to an InputPeer.
// Channels use positive MTProto IDs; groups use negative IDs.
func idToPeer(id int64, accessHash int64) tg.InputPeerClass {
	if id < 0 {
		return &tg.InputPeerChat{ChatID: -id}
	}
	return &tg.InputPeerUser{UserID: id, AccessHash: accessHash}
}

func peerToInput(p pipeline.Peer) tg.InputPeerClass {
	switch p.Type {
	case "chat":
		return &tg.InputPeerChat{ChatID: -p.ID}
	case "user":
		return &tg.InputPeerUser{UserID: p.ID, AccessHash: p.AccessHash}
	default: // "channel"
		return &tg.InputPeerChannel{ChannelID: p.ID, AccessHash: p.AccessHash}
	}
}

func randomID() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return int64(binary.LittleEndian.Uint64(b[:]))
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/client/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/client/
git commit -m "feat: add client package wrapping gotd/td for auth, receive, send, forward"
```

---

## Task 7: Bot handler

**Files:**
- Modify: `internal/bot/bot.go` (add `Handler` struct and `dispatch`)

- [ ] **Step 1: Replace internal/bot/bot.go with the complete final file**

Overwrite `internal/bot/bot.go` entirely with:

```go
package bot

import (
	"context"
	"fmt"
	"strings"

	"feedwatch/internal/config"
	"feedwatch/internal/pipeline"
)

// ParseCommand parses a Telegram command text (e.g. "/add_run my-feed golang").
// Returns command name (without /), args slice, and whether it is a command.
func ParseCommand(text string) (name string, args []string, ok bool) {
	if len(text) == 0 || text[0] != '/' {
		return "", nil, false
	}
	parts := strings.Fields(text)
	name = strings.TrimPrefix(parts[0], "/")
	if name == "" {
		return "", nil, false
	}
	return name, parts[1:], true
}

// PeerResolver is satisfied by *client.Client.
type PeerResolver interface {
	ResolvePeer(ctx context.Context, username string) (pipeline.Peer, error)
}

type Handler struct {
	cfg      *config.Config
	resolver PeerResolver
}

func NewHandler(cfg *config.Config, resolver PeerResolver) *Handler {
	return &Handler{cfg: cfg, resolver: resolver}
}

// Handle dispatches a bot command from the owner and returns a reply string.
// Returns "" if text is not a command.
func (h *Handler) Handle(ctx context.Context, text string) string {
	name, args, ok := ParseCommand(text)
	if !ok {
		return ""
	}
	reply, err := h.dispatch(ctx, name, args)
	if err != nil {
		return "error: " + err.Error()
	}
	return reply
}

func (h *Handler) dispatch(ctx context.Context, name string, args []string) (string, error) {
	switch name {
	case "new_pipeline":
		if len(args) != 1 {
			return "", fmt.Errorf("usage: /new_pipeline <name>")
		}
		id := args[0]
		if _, exists := h.cfg.FindPipeline(id); exists {
			return "", fmt.Errorf("pipeline %q already exists", id)
		}
		h.cfg.AddPipeline(pipeline.Pipeline{ID: id})
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("save: %w", err)
		}
		return fmt.Sprintf("created pipeline %q", id), nil

	case "add_source":
		if len(args) != 2 {
			return "", fmt.Errorf("usage: /add_source <pipeline> @username")
		}
		pipelineID, username := args[0], args[1]
		p, ok := h.cfg.FindPipeline(pipelineID)
		if !ok {
			return "", fmt.Errorf("pipeline %q not found", pipelineID)
		}
		peer, err := h.resolver.ResolvePeer(ctx, username)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", username, err)
		}
		p.Sources = append(p.Sources, peer)
		h.cfg.UpdatePipeline(p)
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("save: %w", err)
		}
		return fmt.Sprintf("added source %s (id=%d) to %q", username, peer.ID, pipelineID), nil

	case "set_output":
		if len(args) != 2 {
			return "", fmt.Errorf("usage: /set_output <pipeline> @username")
		}
		pipelineID, username := args[0], args[1]
		p, ok := h.cfg.FindPipeline(pipelineID)
		if !ok {
			return "", fmt.Errorf("pipeline %q not found", pipelineID)
		}
		peer, err := h.resolver.ResolvePeer(ctx, username)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", username, err)
		}
		p.Output = &peer
		h.cfg.UpdatePipeline(p)
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("save: %w", err)
		}
		return fmt.Sprintf("set output to %s (id=%d) for %q", username, peer.ID, pipelineID), nil

	case "add_run":
		if len(args) != 2 {
			return "", fmt.Errorf("usage: /add_run <pipeline> <word>")
		}
		pipelineID, word := args[0], args[1]
		p, ok := h.cfg.FindPipeline(pipelineID)
		if !ok {
			return "", fmt.Errorf("pipeline %q not found", pipelineID)
		}
		p.Filter.RunWords = append(p.Filter.RunWords, word)
		h.cfg.UpdatePipeline(p)
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("save: %w", err)
		}
		return fmt.Sprintf("added run-word %q to %q", word, pipelineID), nil

	case "add_stop":
		if len(args) != 2 {
			return "", fmt.Errorf("usage: /add_stop <pipeline> <word>")
		}
		pipelineID, word := args[0], args[1]
		p, ok := h.cfg.FindPipeline(pipelineID)
		if !ok {
			return "", fmt.Errorf("pipeline %q not found", pipelineID)
		}
		p.Filter.StopWords = append(p.Filter.StopWords, word)
		h.cfg.UpdatePipeline(p)
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("save: %w", err)
		}
		return fmt.Sprintf("added stop-word %q to %q", word, pipelineID), nil

	case "del_pipeline":
		if len(args) != 1 {
			return "", fmt.Errorf("usage: /del_pipeline <pipeline>")
		}
		id := args[0]
		if !h.cfg.DeletePipeline(id) {
			return "", fmt.Errorf("pipeline %q not found", id)
		}
		if err := h.cfg.Save(); err != nil {
			return "", fmt.Errorf("save: %w", err)
		}
		return fmt.Sprintf("deleted pipeline %q", id), nil

	case "list":
		ps := h.cfg.Pipelines()
		if len(ps) == 0 {
			return "no pipelines configured", nil
		}
		var sb strings.Builder
		for _, p := range ps {
			sb.WriteString(fmt.Sprintf("[%s]\n", p.ID))
			sb.WriteString(fmt.Sprintf("  sources: %d\n", len(p.Sources)))
			if p.Output != nil {
				sb.WriteString(fmt.Sprintf("  output: id=%d\n", p.Output.ID))
			} else {
				sb.WriteString("  output: not set\n")
			}
			if len(p.Filter.RunWords) > 0 {
				sb.WriteString(fmt.Sprintf("  run: %s\n", strings.Join(p.Filter.RunWords, ", ")))
			}
			if len(p.Filter.StopWords) > 0 {
				sb.WriteString(fmt.Sprintf("  stop: %s\n", strings.Join(p.Filter.StopWords, ", ")))
			}
		}
		return sb.String(), nil

	default:
		return fmt.Sprintf(
			"unknown command: /%s\ncommands: /new_pipeline /add_source /set_output /add_run /add_stop /del_pipeline /list",
			name,
		), nil
	}
}

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/bot/...
```

Expected: no errors.

- [ ] **Step 3: Run all tests to ensure nothing broke**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/bot/bot.go
git commit -m "feat: add bot Handler with full command dispatch"
```

---

## Task 8: Main entry point

**Files:**
- Create: `cmd/feedwatch/main.go`

- [ ] **Step 1: Create main.go**

Create `cmd/feedwatch/main.go`:

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"feedwatch/internal/bot"
	"feedwatch/internal/client"
	"feedwatch/internal/config"
	"feedwatch/internal/pipeline"
)

func main() {
	appIDStr := os.Getenv("APP_ID")
	appHash := os.Getenv("APP_HASH")
	if appIDStr == "" || appHash == "" {
		log.Fatal("APP_ID and APP_HASH environment variables must be set\n" +
			"Get them at https://my.telegram.org/apps")
	}
	appID, err := strconv.Atoi(appIDStr)
	if err != nil {
		log.Fatalf("invalid APP_ID %q: %v", appIDStr, err)
	}

	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	c := client.New(appID, appHash, "session.json", cfg.OwnerID())
	handler := bot.NewHandler(cfg, c)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err = c.Run(ctx, func(ctx context.Context, event client.MessageEvent) {
		if event.IsOwnerCommand {
			reply := handler.Handle(ctx, event.Text)
			if reply == "" {
				return
			}
			if sendErr := c.SendMessage(ctx, event.ChatID, event.ChatAccessHash, reply); sendErr != nil {
				log.Printf("send reply: %v", sendErr)
			}
			return
		}

		if event.Text == "" {
			return // skip non-text messages (photos, stickers, etc.)
		}

		matched := pipeline.NewRouter(cfg.Pipelines()).Route(event.ChatID, event.Text)
		for _, p := range matched {
			if p.Output == nil {
				continue
			}
			fromPeer := pipeline.Peer{
				ID:         event.ChatID,
				AccessHash: event.ChatAccessHash,
				Type:       event.ChatType,
			}
			if fwdErr := c.ForwardMessage(ctx, fromPeer, event.MessageID, *p.Output); fwdErr != nil {
				log.Printf("forward pipeline %q to %d: %v", p.ID, p.Output.ID, fwdErr)
			}
		}
	})

	if err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Build the binary**

```bash
go build ./cmd/feedwatch/...
```

Expected: no errors, `feedwatch` binary created.

- [ ] **Step 3: Run all tests one final time**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/feedwatch/main.go
git commit -m "feat: add main entry point, wire all packages"
```

---

## First Run

```bash
# Get API credentials from https://my.telegram.org/apps
export APP_ID=12345
export APP_HASH=your_hash_here

./feedwatch
# Prompts for phone number, then SMS code, then 2FA password if set.
# On success: "feedwatch: authenticated, listening for messages"
```

Then in Telegram Saved Messages (message yourself):

```
/new_pipeline go-jobs
/add_source go-jobs @golang_jobs_channel
/set_output go-jobs @my_filtered_feed
/add_run go-jobs golang
/add_run go-jobs "go developer"
/add_stop go-jobs junior
/list
```

> **Note on source IDs:** Sources and outputs are identified by Telegram username via `@handle`. The resolved MTProto IDs are stored in `config.json` automatically. Do not edit `config.json` by hand while the bot is running.
