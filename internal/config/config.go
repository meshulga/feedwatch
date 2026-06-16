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
