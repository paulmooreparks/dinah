package requestsettlement

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

// UnmarshalJSON rejects malformed scalar values before encoding/json can replace
// them. Authority fields are never accepted in a settlement input.
func (in *Input) UnmarshalJSON(raw []byte) error {
	bad := errors.New("invalid settlement input")
	if len(raw) > 16384 || !utf8.Valid(raw) || !json.Valid(raw) || !pairedSurrogates(raw) {
		return bad
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	opening, err := d.Token()
	if err != nil || opening != json.Delim('{') {
		return bad
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return bad
		}
		name, ok := key.(string)
		if !ok || fields[name] != nil || (name != "id" && name != "revision" && name != "operationId") {
			return bad
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return bad
		}
		fields[name] = value
	}
	if _, err := d.Token(); err != nil {
		return bad
	}
	if _, err := d.Token(); err != io.EOF || len(fields) != 3 {
		return bad
	}
	var out Input
	for name, target := range map[string]*string{"id": &out.ID, "operationId": &out.OperationID} {
		value := fields[name]
		if len(value) == 0 || value[0] != '"' || json.Unmarshal(value, target) != nil {
			return bad
		}
	}
	if bytes.Equal(fields["revision"], []byte("null")) || json.Unmarshal(fields["revision"], &out.Revision) != nil {
		return bad
	}
	if !ValidOperationID(out.OperationID) || out.ID == "" || out.Revision < 1 {
		return bad
	}
	*in = out
	return nil
}

// pairedSurrogates is called only after json.Valid has established escape bounds.
func pairedSurrogates(raw []byte) bool {
	quoted := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		n, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n < 0xd800 || n > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || string(raw[i+1:i+3]) != `\u` {
			return false
		}
		low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return true
}
