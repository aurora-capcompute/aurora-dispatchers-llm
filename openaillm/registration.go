package openaillm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aurora-capcompute/aurora-dispatchers/builtin"
	"github.com/aurora-capcompute/aurora-dispatchers/registry"
	"github.com/aurora-capcompute/capcompute/dispatcher"
)

var validOperations = map[string]struct{}{
	"openai.chat":        {},
	"openai.responses":   {},
	"openai.embeddings":  {},
	"openai.models.list": {},
}

type Registration struct{}

func (Registration) Matches(name string) bool {
	_, ok := validOperations[name]
	return ok
}

func (Registration) Normalize(name string, raw json.RawMessage) (json.RawMessage, error) {
	if _, ok := validOperations[name]; !ok {
		return nil, fmt.Errorf("unsupported OpenAI-compatible operation %q", name)
	}
	var settings Settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &settings); err != nil {
			return nil, err
		}
	}
	normalized, err := normalizeSettings(settings)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized.Settings)
}

func (Registration) Configure(
	_ context.Context,
	name string,
	raw json.RawMessage,
	_ registry.Services,
	config *builtin.Config,
) error {
	normalizedRaw, err := (Registration{}).Normalize(name, raw)
	if err != nil {
		return err
	}
	var settings Settings
	if err := json.Unmarshal(normalizedRaw, &settings); err != nil {
		return err
	}
	normalized, err := normalizeSettings(settings)
	if err != nil {
		return err
	}
	handler, err := findOrCreateHandler(config, normalized)
	if err != nil {
		return err
	}
	handler.AddCapability(name, normalized)
	config.Capabilities = append(config.Capabilities, capabilityFor(name, normalized))
	return nil
}

func (Registration) IsSubset(name string, parent, child json.RawMessage) error {
	var parentSettings, childSettings Settings
	if err := json.Unmarshal(parent, &parentSettings); err != nil {
		return fmt.Errorf("decode parent settings: %w", err)
	}
	if err := json.Unmarshal(child, &childSettings); err != nil {
		return fmt.Errorf("decode child settings: %w", err)
	}
	if parentSettings.BaseURL != "" && childSettings.BaseURL != "" && parentSettings.BaseURL != childSettings.BaseURL {
		return fmt.Errorf("child base_url %q differs from parent %q", childSettings.BaseURL, parentSettings.BaseURL)
	}
	if len(parentSettings.AllowedModels) > 0 {
		allowed := make(map[string]struct{}, len(parentSettings.AllowedModels))
		for _, m := range parentSettings.AllowedModels {
			allowed[m] = struct{}{}
		}
		for _, m := range childSettings.AllowedModels {
			if _, ok := allowed[m]; !ok {
				return fmt.Errorf("child model %q is not in parent's allowed models", m)
			}
		}
	}
	return nil
}

func findOrCreateHandler(config *builtin.Config, settings normalizedSettings) (*Handler, error) {
	connection := connectionFor(settings)
	for _, existing := range config.Handlers {
		if handler, ok := existing.(*Handler); ok {
			if handler.connection != connection {
				return nil, fmt.Errorf("OpenAI-compatible capabilities must use identical connection settings")
			}
			return handler, nil
		}
	}
	client, err := NewClient(settings)
	if err != nil {
		client = failedClient{err: err}
	}
	handler := NewHandler(client)
	handler.connection = connection
	config.Handlers = append(config.Handlers, handler)
	return handler, nil
}

func connectionFor(settings normalizedSettings) connectionSettings {
	maxRetries := 0
	maxRetriesSet := settings.MaxRetries != nil
	if maxRetriesSet {
		maxRetries = *settings.MaxRetries
	}
	headers := make([]string, 0, len(settings.Headers))
	for header, value := range settings.Headers {
		headers = append(headers, header+"="+value)
	}
	sort.Strings(headers)
	return connectionSettings{
		baseURL:        settings.BaseURL,
		apiKey:         settings.APIKey,
		apiKeyOptional: settings.APIKeyOptional,
		organization:   settings.Organization,
		project:        settings.Project,
		timeout:        settings.timeout.String(),
		maxRetries:     maxRetries,
		maxRetriesSet:  maxRetriesSet,
		insecureHTTP:   settings.AllowInsecureHTTP,
		headers:        strings.Join(headers, "\n"),
	}
}

type failedClient struct{ err error }

func (c failedClient) Chat(context.Context, json.RawMessage) (json.RawMessage, error) {
	return nil, c.err
}
func (c failedClient) Responses(context.Context, json.RawMessage) (json.RawMessage, error) {
	return nil, c.err
}
func (c failedClient) Embeddings(context.Context, json.RawMessage) (json.RawMessage, error) {
	return nil, c.err
}
func (c failedClient) Models(context.Context) (json.RawMessage, error) {
	return nil, c.err
}

func capabilityFor(name string, settings normalizedSettings) dispatcher.Capability {
	models := "all models"
	if len(settings.AllowedModels) > 0 {
		models = "models: " + strings.Join(settings.AllowedModels, ", ")
	}
	approvalNote := ""
	if requiresApproval(name, settings.Settings) {
		approvalNote = " Requires human approval."
	}
	descriptions := map[string]string{
		"openai.chat":        "Create a Chat Completions response.",
		"openai.responses":   "Create a Responses API response.",
		"openai.embeddings":  "Create embeddings.",
		"openai.models.list": "List provider models.",
	}
	return dispatcher.Capability{
		Name:        name,
		Description: fmt.Sprintf("%s Provider: %s; %s.%s", descriptions[name], settings.BaseURL, models, approvalNote),
		InputSchema: schemas[name],
	}
}

var schemas = map[string]json.RawMessage{
	"openai.chat":        json.RawMessage(`{"type":"object","properties":{"model":{"type":"string"},"messages":{"type":"array","items":{"type":"object","properties":{"role":{"type":"string"},"content":{}},"required":["role","content"],"additionalProperties":true}},"temperature":{"type":"number","minimum":0,"maximum":2},"top_p":{"type":"number","minimum":0,"maximum":1},"max_tokens":{"type":"integer","minimum":1},"max_completion_tokens":{"type":"integer","minimum":1},"response_format":{"type":"object"},"tools":{"type":"array"},"tool_choice":{},"stream":{"const":false}},"required":["messages"],"additionalProperties":true}`),
	"openai.responses":   json.RawMessage(`{"type":"object","properties":{"model":{"type":"string"},"input":{},"instructions":{"type":"string"},"max_output_tokens":{"type":"integer","minimum":1},"temperature":{"type":"number","minimum":0,"maximum":2},"tools":{"type":"array"},"stream":{"const":false}},"required":["input"],"additionalProperties":true}`),
	"openai.embeddings":  json.RawMessage(`{"type":"object","properties":{"model":{"type":"string"},"input":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"}}]},"dimensions":{"type":"integer","minimum":1},"encoding_format":{"enum":["float","base64"]}},"required":["input"],"additionalProperties":true}`),
	"openai.models.list": json.RawMessage(`{"type":"object","additionalProperties":false}`),
}
