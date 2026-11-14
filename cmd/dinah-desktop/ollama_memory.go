package main

import (
	"context"
	"time"

	"studio/internal/ollamaagent"
)

// OllamaMemoryReading is a read-only sample of currently loaded local models.
type OllamaMemoryReading struct {
	Limit  int                       `json:"limit"`
	Models []ollamaagent.ModelMemory `json:"models"`
}

// OllamaMemory reads the documented ps and show APIs. It never loads a model.
func (a *App) OllamaMemory() (OllamaMemoryReading, error) {
	a.mu.Lock()
	limit := ollamaContextCap(a.settings)
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	models, err := ollamaagent.Memory(ctx, ollamaagent.Host(), limit)
	return OllamaMemoryReading{Limit: limit, Models: models}, err
}
