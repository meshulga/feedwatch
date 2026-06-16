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
