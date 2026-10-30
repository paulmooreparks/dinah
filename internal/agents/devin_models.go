package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os/exec"
	"time"

	"studio/internal/acp"
)

// Devin's model prices come from `devin models list --format json`. Devin
// documents the command, and documents --format json as machine-readable
// output for scripts (https://docs.devin.ai/cli/reference/commands), but it
// does not document the fields of that output. Paul accepted relying on
// them on 28 October 2026, as an exception to the rule against relying on
// undocumented behaviour, on the condition that every way it can go wrong
// leaves the model list as it would be with no prices at all (DESIGN.md,
// Devin Model Prices). So nothing here returns a price unless the output
// has the shape seen on Devin 3000.4.25:
//
//	{"families": [{..., "variants": [{"model_uid": "...", "label": "...",
//	  "max_context_tokens": 262000, "cost_tier": "High cost",
//	  "cost_summary": "$0.5 / MTok In · $2.5 / MTok Out", ...}]}]}
//
// A field of another type is ignored, a variant without a model_uid or a
// cost_summary is left out, and output that is not this shape at all gives
// no prices. Prices are matched to the models Devin offers over the Agent
// Client Protocol by exact equality of model_uid with the option's value,
// which is what Devin 3000.4.25 sent for all 82 models it offered.

// DevinModelPrice is what Devin's command line says one model costs.
type DevinModelPrice struct {
	// CostSummary is Devin's own words for the price, such as
	// "$4 / MTok In · $20 / MTok Out"; a price always has one.
	CostSummary string `json:"costSummary"`
	// CostTier is Devin's rough grade, such as "High cost", when it gives
	// one.
	CostTier string `json:"costTier,omitempty"`
	// ContextTokens is the model's context window in tokens, or 0 when Devin
	// does not say.
	ContextTokens int64 `json:"contextTokens,omitempty"`
}

// devinModelsTimeout bounds how long `devin models list` may take. It took
// about two seconds on 28 October 2026, and nothing waits for it.
var devinModelsTimeout = 8 * time.Second

// devinModelsLimit is the most output read from the command; it printed
// about 90 KB on 28 October 2026.
const devinModelsLimit = 4 << 20

// runDevinModels runs `devin models list --format json` with no window and
// returns what it printed; tests replace it, so that they never start Devin.
var runDevinModels = func(ctx context.Context, path string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, "models", "list", "--format", "json")
	acp.HideWindow(cmd)
	out := &limitedBuffer{limit: devinModelsLimit}
	cmd.Stdout = out
	// A process Devin starts could keep the output open after Devin is
	// ended at the timeout; WaitDelay stops the wait for it.
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if out.over {
		return nil, errors.New("the model list was larger than expected")
	}
	return out.Bytes(), nil
}

// limitedBuffer keeps at most limit bytes and notes whether more came.
type limitedBuffer struct {
	bytes.Buffer
	limit int
	over  bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.Len(); len(p) > room {
		b.over = true
		if room > 0 {
			b.Buffer.Write(p[:room])
		}
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

// DevinModelPrices runs the Devin command line at path and returns the
// prices it lists, by model_uid. An error means there are no prices to show,
// whatever the cause.
func DevinModelPrices(path string) (map[string]DevinModelPrice, error) {
	ctx, cancel := context.WithTimeout(context.Background(), devinModelsTimeout)
	defer cancel()
	out, err := runDevinModels(ctx, path)
	if ctx.Err() != nil {
		return nil, errors.New("the Devin command line did not list its models in time")
	}
	if err != nil {
		return nil, err
	}
	prices := ParseDevinModels(out)
	if len(prices) == 0 {
		return nil, errors.New("the Devin command line listed no prices that could be read")
	}
	return prices, nil
}

// ParseDevinModels reads the output of `devin models list --format json`
// and returns the prices in it, by model_uid. It never fails: what it cannot
// read gives no price.
func ParseDevinModels(data []byte) map[string]DevinModelPrice {
	var top map[string]any
	if json.Unmarshal(data, &top) != nil {
		return nil
	}
	families, _ := top["families"].([]any)
	out := map[string]DevinModelPrice{}
	for _, f := range families {
		family, _ := f.(map[string]any)
		variants, _ := family["variants"].([]any)
		for _, v := range variants {
			variant, _ := v.(map[string]any)
			uid, _ := variant["model_uid"].(string)
			summary, _ := variant["cost_summary"].(string)
			if uid == "" || summary == "" {
				continue
			}
			price := DevinModelPrice{CostSummary: summary}
			price.CostTier, _ = variant["cost_tier"].(string)
			if n, ok := variant["max_context_tokens"].(float64); ok && n > 0 && n <= 1e12 && n == math.Trunc(n) {
				price.ContextTokens = int64(n)
			}
			out[uid] = price
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// MatchDevinPrices keeps the prices of the models the options offer, matched
// by exact equality of the model option's value with model_uid. A model with
// no price under its own value gets none.
func MatchDevinPrices(opts []acp.ConfigOption, prices map[string]DevinModelPrice) map[string]DevinModelPrice {
	model, ok := ModelOption(opts)
	if !ok || len(prices) == 0 {
		return nil
	}
	out := map[string]DevinModelPrice{}
	for _, value := range OptionValues(model) {
		if p, ok := prices[value]; ok {
			out[value] = p
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
