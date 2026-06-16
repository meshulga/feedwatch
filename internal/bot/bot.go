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
