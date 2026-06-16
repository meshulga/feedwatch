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
	if f.Matches("junior golang position") {
		t.Error("stop-word should block even when run-word present")
	}
}
