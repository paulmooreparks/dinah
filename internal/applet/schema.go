package applet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const MaxJSONBytes = 1 << 20

type closedLoader struct{}

func (closedLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema references are unavailable: %s", url)
}

func decodeJSON(document string) (any, error) {
	if len(document) > MaxJSONBytes {
		return nil, errors.New("applet JSON exceeds 1 MB")
	}
	d := json.NewDecoder(strings.NewReader(document))
	d.UseNumber()
	var value any
	if err := d.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, errors.New("expected one JSON value")
	}
	return value, nil
}

func compileSchema(schema json.RawMessage) (*jsonschema.Schema, error) {
	doc, err := decodeJSON(string(schema))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseLoader(closedLoader{})
	if err = c.AddResource("urn:dinah:applet-schema", doc); err != nil {
		return nil, err
	}
	return c.Compile("urn:dinah:applet-schema")
}

// Validate preserves JSON numbers and never loads schemas from files or URLs.
func Validate(schema json.RawMessage, document string) error {
	s, err := compileSchema(bytes.Clone(schema))
	if err != nil {
		return fmt.Errorf("invalid action schema: %w", err)
	}
	v, err := decodeJSON(document)
	if err != nil {
		return err
	}
	return s.Validate(v)
}
