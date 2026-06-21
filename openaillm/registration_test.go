package openaillm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCapabilitySchemasAreValidJSON(t *testing.T) {
	for name, schema := range schemas {
		if !json.Valid(schema) {
			t.Fatalf("%s schema is invalid JSON", name)
		}
	}
}

func TestNormalizeRejectsUnknownCapability(t *testing.T) {
	_, err := (Registration{}).Normalize("openai.unknown", nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %v", err)
	}
}
