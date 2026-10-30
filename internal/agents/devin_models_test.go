package agents

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
)

// sampleDevinModels is the shape `devin models list --format json` printed
// on Devin 3000.4.25, cut to three families.
const sampleDevinModels = `{
  "families": [
    {"family_label": "Adaptive", "family_uid": "Adaptive", "slug": "adaptive", "aliases": [],
     "variants": [{"model_uid": "adaptive", "label": "Adaptive", "description": "Automatically balances quality and cost", "is_new": false, "is_beta": false}]},
    {"family_label": "SWE-2", "family_uid": "swe-2", "slug": "swe-2", "aliases": ["swe"],
     "variants": [
       {"model_uid": "swe-2-high", "label": "SWE-2 High", "max_context_tokens": 262000, "max_output_tokens": 128000, "cost_tier": "High cost", "cost_summary": "$0.5 / MTok In · $2.5 / MTok Out", "is_new": false, "is_beta": false},
       {"model_uid": "swe-2-medium", "label": "SWE-2 Medium", "max_context_tokens": 262000, "max_output_tokens": 128000, "cost_tier": "High cost", "cost_summary": "$0.5 / MTok In · $2.5 / MTok Out", "is_new": false, "is_beta": false}]},
    {"family_label": "Claude Opus 5.5", "family_uid": "claude-opus-5-5", "slug": "claude-opus-5.5", "aliases": ["opus"],
     "variants": [{"model_uid": "claude-opus-5-5-medium", "label": "Claude Opus 5.5 Medium", "max_context_tokens": 1000000, "max_output_tokens": 128000, "cost_tier": "High cost", "cost_summary": "$4 / MTok In · $20 / MTok Out", "is_new": false, "is_beta": false}]}
  ]
}`

func TestParseDevinModelsReadsTheSample(t *testing.T) {
	got := ParseDevinModels([]byte(sampleDevinModels))
	want := map[string]DevinModelPrice{
		"swe-2-high":             {CostSummary: "$0.5 / MTok In · $2.5 / MTok Out", CostTier: "High cost", ContextTokens: 262000},
		"swe-2-medium":           {CostSummary: "$0.5 / MTok In · $2.5 / MTok Out", CostTier: "High cost", ContextTokens: 262000},
		"claude-opus-5-5-medium": {CostSummary: "$4 / MTok In · $20 / MTok Out", CostTier: "High cost", ContextTokens: 1000000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

// A variant without a cost_summary, such as Adaptive, has no price; one
// without a tier or a context size still has its summary.
func TestParseDevinModelsWithMissingFields(t *testing.T) {
	got := ParseDevinModels([]byte(`{"families":[{"variants":[
		{"model_uid":"a"},
		{"model_uid":"b","cost_summary":"$1 / MTok In"},
		{"cost_summary":"$2 / MTok In"},
		{"model_uid":"","cost_summary":"$3 / MTok In"}]}]}`))
	want := map[string]DevinModelPrice{"b": {CostSummary: "$1 / MTok In"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

// Only fields of the expected types are used: a field of another type is
// ignored, and a variant whose model_uid or cost_summary is not a string has
// no price.
func TestParseDevinModelsIgnoresWrongTypes(t *testing.T) {
	got := ParseDevinModels([]byte(`{"families":[
		{"variants":[
			{"model_uid":7,"cost_summary":"$1"},
			{"model_uid":"x","cost_summary":{"in":1}},
			{"model_uid":"y","cost_summary":"$2","cost_tier":3,"max_context_tokens":"262000"},
			{"model_uid":"z","cost_summary":"$3","max_context_tokens":-5},
			{"model_uid":"w","cost_summary":"$4","max_context_tokens":1.5},
			"not a variant", null, 12]},
		{"variants":"not a list"},
		"not a family"]}`))
	want := map[string]DevinModelPrice{"y": {CostSummary: "$2"}, "z": {CostSummary: "$3"}, "w": {CostSummary: "$4"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	for _, garbage := range []string{``, `not json`, `[]`, `null`, `{"families":{}}`, `{"families":[]}`, `{"models":[{"model_uid":"a","cost_summary":"$1"}]}`, `{"families":[{"variants":[{"model_uid":"a","cost_summary":"$1"`} {
		if got := ParseDevinModels([]byte(garbage)); got != nil {
			t.Errorf("%q gave %+v", garbage, got)
		}
	}
}

func withDevinModels(t *testing.T, run func(context.Context, string) ([]byte, error)) {
	t.Helper()
	saved, savedTimeout := runDevinModels, devinModelsTimeout
	t.Cleanup(func() { runDevinModels, devinModelsTimeout = saved, savedTimeout })
	runDevinModels = run
}

func TestDevinModelPricesRunsTheCommandLine(t *testing.T) {
	var ran string
	withDevinModels(t, func(_ context.Context, path string) ([]byte, error) {
		ran = path
		return []byte(sampleDevinModels), nil
	})
	prices, err := DevinModelPrices(`C:\devin.exe`)
	if err != nil || len(prices) != 3 || ran != `C:\devin.exe` {
		t.Fatalf("prices %+v, err %v, ran %q", prices, err, ran)
	}
}

func TestDevinModelPricesFailsQuietly(t *testing.T) {
	cases := map[string]func(context.Context, string) ([]byte, error){
		"a failing command": func(context.Context, string) ([]byte, error) {
			return nil, errors.New("exit status 1")
		},
		"garbage": func(context.Context, string) ([]byte, error) { return []byte("Error: not signed in"), nil },
		"no prices": func(context.Context, string) ([]byte, error) {
			return []byte(`{"families":[{"variants":[{"model_uid":"adaptive"}]}]}`), nil
		},
		"a timeout": func(ctx context.Context, _ string) ([]byte, error) {
			<-ctx.Done()
			return []byte(sampleDevinModels), ctx.Err()
		},
	}
	for name, run := range cases {
		withDevinModels(t, run)
		devinModelsTimeout = 50 * time.Millisecond
		started := time.Now()
		prices, err := DevinModelPrices("devin")
		if err == nil || prices != nil {
			t.Errorf("%s: prices %+v, err %v", name, prices, err)
		}
		if time.Since(started) > 5*time.Second {
			t.Errorf("%s took %v", name, time.Since(started))
		}
	}
}

// The command itself, run for a program that is not there, fails quietly
// too.
func TestDevinModelPricesWithNoProgram(t *testing.T) {
	prices, err := DevinModelPrices(t.TempDir() + `\no-devin.exe`)
	if err == nil || prices != nil {
		t.Fatalf("prices %+v, err %v", prices, err)
	}
}

func modelOptions(t *testing.T, values ...string) []acp.ConfigOption {
	t.Helper()
	var list []map[string]string
	for _, v := range values {
		list = append(list, map[string]string{"value": v, "name": strings.ToUpper(v)})
	}
	raw, _ := json.Marshal(list)
	return []acp.ConfigOption{
		{ID: "mode", Category: "mode", Type: "select", Options: json.RawMessage(`[{"value":"swe-2-high","name":"Not a model"}]`)},
		{ID: "model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"adaptive"`), Options: raw},
	}
}

// A model gets the price listed under its own value, and no other: a model
// Devin lists no price for, or one only an alias or a slug would match, gets
// none.
func TestMatchDevinPricesByExactModelID(t *testing.T) {
	prices := ParseDevinModels([]byte(sampleDevinModels))
	got := MatchDevinPrices(modelOptions(t, "adaptive", "swe-2-high", "opus", "claude-opus-5.5", "Claude-Opus-5-5-Medium", "claude-opus-5-5-medium"), prices)
	want := map[string]DevinModelPrice{"swe-2-high": prices["swe-2-high"], "claude-opus-5-5-medium": prices["claude-opus-5-5-medium"]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if got := MatchDevinPrices(modelOptions(t, "adaptive", "unknown"), prices); got != nil {
		t.Fatalf("unmatched models got %+v", got)
	}
	if got := MatchDevinPrices(nil, prices); got != nil {
		t.Fatalf("no options got %+v", got)
	}
	if got := MatchDevinPrices(modelOptions(t, "swe-2-high"), nil); got != nil {
		t.Fatalf("no prices got %+v", got)
	}
}

// Grouped model values, which the protocol allows, are matched too.
func TestMatchDevinPricesInGroups(t *testing.T) {
	prices := ParseDevinModels([]byte(sampleDevinModels))
	opts := []acp.ConfigOption{{ID: "model", Category: "model", Type: "select", Options: json.RawMessage(`[{"group":"g","name":"G","options":[{"value":"swe-2-medium","name":"SWE-2 Medium"}]}]`)}}
	if got := MatchDevinPrices(opts, prices); len(got) != 1 || got["swe-2-medium"].CostTier != "High cost" {
		t.Fatalf("got %+v", got)
	}
}
