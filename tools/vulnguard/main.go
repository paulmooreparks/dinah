// Vulnguard checks govulncheck's documented streaming JSON. JSON mode returns
// success even when it reports called vulnerabilities, so CI must inspect it.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
)

type scanConfig struct {
	Protocol string `json:"protocol_version"`
	Scanner  string `json:"scanner_name"`
	Version  string `json:"scanner_version"`
	Go       string `json:"go_version"`
	Level    string `json:"scan_level"`
	Mode     string `json:"scan_mode"`
}

type scanBOM struct {
	Go    string   `json:"go_version"`
	Roots []string `json:"roots"`
}

type scanFinding struct {
	ID    string `json:"osv"`
	Trace []struct {
		Module   string `json:"module"`
		Package  string `json:"package"`
		Function string `json:"function"`
	} `json:"trace"`
}

func check(r io.Reader, root, version string) ([]string, error) {
	d := json.NewDecoder(r)
	configured, bom := false, false
	called := map[string]bool{}
	for {
		var message map[string]json.RawMessage
		if err := d.Decode(&message); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("invalid scanner JSON: %w", err)
		}
		if len(message) != 1 {
			return nil, errors.New("scanner message must have one field")
		}
		if !configured && message["config"] == nil {
			return nil, errors.New("scanner config must be first")
		}
		for kind, data := range message {
			if string(data) == "null" {
				return nil, errors.New("null scanner message")
			}
			switch kind {
			case "config":
				var c scanConfig
				if err := json.Unmarshal(data, &c); err != nil {
					return nil, err
				}
				if configured || c.Protocol != "v1.0.0" || c.Scanner != "govulncheck" || c.Version != "v1.9.0" || c.Mode != "source" || c.Level != "symbol" || c.Go != version {
					return nil, fmt.Errorf("unexpected scanner config: %+v", c)
				}
				configured = true
			case "SBOM":
				var b scanBOM
				if err := json.Unmarshal(data, &b); err != nil {
					return nil, err
				}
				if bom || b.Go != version || len(b.Roots) != 1 || b.Roots[0] != root {
					return nil, fmt.Errorf("unexpected scanner target: %+v", b)
				}
				bom = true
			case "finding":
				var f scanFinding
				if err := json.Unmarshal(data, &f); err != nil {
					return nil, err
				}
				if f.ID == "" || len(f.Trace) == 0 || f.Trace[0].Module == "" {
					return nil, errors.New("incomplete scanner finding")
				}
				// Module/package matches are informational in a symbol-level
				// source scan. A function in the first frame means a called symbol.
				if f.Trace[0].Function != "" {
					called[f.ID] = true
				}
			case "osv", "progress":
				// These documented message types do not determine reachability.
			default:
				return nil, fmt.Errorf("unsupported scanner message %q", kind)
			}
		}
	}
	if !configured || !bom {
		return nil, errors.New("scanner report lacks config or target SBOM")
	}
	ids := make([]string, 0, len(called))
	for id := range called {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, nil
}

func main() {
	root := flag.String("root", "", "expected single source package root")
	version := flag.String("go-version", "", "required analyzed Go version")
	flag.Parse()
	if *root == "" || *version == "" || flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: vulnguard -root <package> -go-version <version> <report.json>")
		os.Exit(2)
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer f.Close()
	ids, err := check(f, *root, *version)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if len(ids) > 0 {
		fmt.Fprintf(os.Stderr, "%s: called vulnerabilities: %v\n", *root, ids)
		os.Exit(1)
	}
	fmt.Printf("%s: no called vulnerabilities; analyzed with %s\n", *root, *version)
}
