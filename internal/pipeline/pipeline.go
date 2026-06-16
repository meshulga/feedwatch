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
