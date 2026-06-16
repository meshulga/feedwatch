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
