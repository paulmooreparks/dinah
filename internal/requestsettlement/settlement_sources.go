package requestsettlement

import "embed"

//go:embed input.go operation_index.go settlement_sources.go types.go
var settlementSources embed.FS

// SettlementSources identifies the compiled implementation used by the bundled
// request-withdrawal adapter. Test-only fixtures are excluded from its digest.
func SettlementSources() (map[string][]byte, error) {
	entries, err := settlementSources.ReadDir(".")
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, entry := range entries {
		data, err := settlementSources.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		out[entry.Name()] = data
	}
	return out, nil
}
