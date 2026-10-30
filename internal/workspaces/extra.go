package workspaces

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// A source or a pin of a kind this build does not know, which a workspace
// export brought from a build that has it, keeps the fields this build
// does not know, so that saving the registry or exporting the workspace
// again writes them unchanged. The format's rule that a reader ignores the
// fields it does not know is what allows this (docs/design/workspace-export.md).

// knownFields are, by type, the JSON names of a struct's fields.
var knownFields sync.Map

// jsonNames returns the JSON names of the fields of the struct type t.
func jsonNames(t reflect.Type) map[string]bool {
	if names, ok := knownFields.Load(t); ok {
		return names.(map[string]bool)
	}
	names := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if tag == "-" || !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "" {
			name = field.Name
		}
		names[name] = true
	}
	knownFields.Store(t, names)
	return names
}

// unmarshalWithExtra reads data into the struct v points to and returns the
// fields of data that v does not have, or nil when there are none.
func unmarshalWithExtra(data []byte, v any) (map[string]json.RawMessage, error) {
	if err := json.Unmarshal(data, v); err != nil {
		return nil, err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	known := jsonNames(reflect.TypeOf(v).Elem())
	var extra map[string]json.RawMessage
	for name, value := range all {
		// encoding/json matches a field's name without regard to case, so
		// a name that differs from a known one only in case is the known one.
		if known[name] || knownFold(known, name) {
			continue
		}
		if extra == nil {
			extra = map[string]json.RawMessage{}
		}
		extra[name] = append(json.RawMessage{}, value...)
	}
	return extra, nil
}

func knownFold(known map[string]bool, name string) bool {
	for k := range known {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

// marshalWithExtra writes the struct v with the fields in extra that it does
// not have itself, after its own, in the order of their names.
func marshalWithExtra(v any, extra map[string]json.RawMessage) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil || len(extra) == 0 {
		return data, err
	}
	known := jsonNames(reflect.TypeOf(v))
	names := make([]string, 0, len(extra))
	for name := range extra {
		if !known[name] && !knownFold(known, name) && json.Valid(extra[name]) {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return data, nil
	}
	sort.Strings(names)
	var out bytes.Buffer
	out.Write(bytes.TrimSuffix(bytes.TrimSpace(data), []byte("}")))
	first := bytes.Equal(bytes.TrimSpace(data), []byte("{}"))
	for _, name := range names {
		if !first {
			out.WriteByte(',')
		}
		first = false
		key, _ := json.Marshal(name)
		out.Write(key)
		out.WriteByte(':')
		var compact bytes.Buffer
		if err := json.Compact(&compact, extra[name]); err != nil {
			return nil, err
		}
		out.Write(compact.Bytes())
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

// copyExtra copies the fields a source or pin does not know, so that a copy
// can be changed on its own.
func copyExtra(extra map[string]json.RawMessage) map[string]json.RawMessage {
	if extra == nil {
		return nil
	}
	out := make(map[string]json.RawMessage, len(extra))
	for name, value := range extra {
		out[name] = append(json.RawMessage{}, value...)
	}
	return out
}

// extraKey is the fields a source or pin does not know as one string, the
// same for the same fields however their JSON was spaced.
func extraKey(extra map[string]json.RawMessage) string {
	if len(extra) == 0 {
		return "{}"
	}
	compact := make(map[string]json.RawMessage, len(extra))
	for name, value := range extra {
		var b bytes.Buffer
		if json.Compact(&b, value) != nil {
			continue
		}
		compact[name] = b.Bytes()
	}
	// encoding/json writes a map's keys in sorted order.
	data, _ := json.Marshal(compact)
	return string(data)
}

// SameExtra reports whether two sources or pins have the same fields that
// this build does not know.
func SameExtra(a, b map[string]json.RawMessage) bool { return extraKey(a) == extraKey(b) }
