package ollamaagent

import (
	"context"
	"net/http"
)

// ModelMemory contains allocations reported by GET /api/ps, in bytes. Target
// is the automatic local context at the supplied ceiling, not an allocation
// prediction. A reading applies to that target only when Context equals Target.
type ModelMemory struct {
	Name    string `json:"name"`
	Digest  string `json:"digest"`
	Size    int64  `json:"size"`
	VRAM    int64  `json:"sizeVRAM"`
	Context int    `json:"context"`
	Target  int    `json:"targetContext"`
}

// Memory reads allocations without loading a model or sending a prompt.
// Show supplies the model maximum for interpreting the Settings ceiling.
func Memory(ctx context.Context, base string, cap int) ([]ModelMemory, error) {
	c := newClient(base)
	var response struct {
		Models []struct {
			Name    string `json:"name"`
			Model   string `json:"model"`
			Digest  string `json:"digest"`
			Size    int64  `json:"size"`
			VRAM    int64  `json:"size_vram"`
			Context int    `json:"context_length"`
		} `json:"models"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/ps", nil, &response); err != nil {
		return nil, err
	}
	models := []ModelMemory{}
	for _, m := range response.Models {
		name := m.Name
		if name == "" {
			name = m.Model
		}
		if name == "" || IsCloudModel(name) {
			continue
		}
		shown, err := c.show(ctx, name)
		target := 0
		if err == nil {
			target = localContext(0, shown.contextLength(), cap)
		}
		models = append(models, ModelMemory{Name: name, Digest: m.Digest, Size: m.Size,
			VRAM: m.VRAM, Context: m.Context, Target: target})
	}
	return models, nil
}
