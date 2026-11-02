package main

import (
	"strings"
	"testing"
)

const config = `{"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck","scanner_version":"v1.9.0","go_version":"go1.26.10","scan_level":"symbol","scan_mode":"source"}}`
const bom = `{"SBOM":{"go_version":"go1.26.10","roots":["studio/cmd/dinah"]}}`
const finding = `{"finding":{"osv":"GO-test","trace":[{"module":"stdlib","package":"net/http","function":"Read"}]}}`

func TestCalledFindingFailsRegardlessOfScannerExit(t *testing.T) {
	ids, err := check(strings.NewReader(config+bom+finding+finding), "studio/cmd/dinah", "go1.26.10")
	if err != nil || len(ids) != 1 || ids[0] != "GO-test" {
		t.Fatalf("called findings = %v, error = %v", ids, err)
	}
}

func TestModuleOnlyFindingDoesNotInventACall(t *testing.T) {
	input := config + bom + `{"osv":{"id":"GO-test"}}` + `{"finding":{"osv":"GO-test","trace":[{"module":"stdlib"}]}}`
	ids, err := check(strings.NewReader(input), "studio/cmd/dinah", "go1.26.10")
	if err != nil || len(ids) != 0 {
		t.Fatalf("module-only findings = %v, error = %v", ids, err)
	}
}

func TestInvalidOrWrongScanCannotPass(t *testing.T) {
	for name, input := range map[string]string{
		"empty":            "",
		"no config":        bom,
		"no target":        config,
		"truncated":        config + bom + `{"finding":`,
		"invalid finding":  config + bom + `{"finding":{}}`,
		"null finding":     config + bom + `{"finding":null}`,
		"unknown message":  config + bom + `{"error":"scan failed"}`,
		"wrong protocol":   strings.Replace(config, "v1.0.0", "v2.0.0", 1) + bom,
		"wrong scanner":    strings.Replace(config, "v1.9.0", "v1.3.0", 1) + bom,
		"wrong toolchain":  strings.ReplaceAll(config+bom, "go1.26.10", "go1.26.0"),
		"wrong root":       config + strings.Replace(bom, "studio/cmd/dinah", "other/package", 1),
		"binary scan":      strings.Replace(config, "source", "binary", 1) + bom,
		"module scan":      strings.Replace(config, "symbol", "module", 1) + bom,
		"duplicate config": config + bom + config,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := check(strings.NewReader(input), "studio/cmd/dinah", "go1.26.10"); err == nil {
				t.Fatal("invalid report passed")
			}
		})
	}
}
