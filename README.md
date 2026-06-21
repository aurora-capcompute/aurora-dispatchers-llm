# aurora-dispatchers-llm

OpenAI-compatible LLM capabilities for Aurora, implemented with the official
[`openai-go`](https://github.com/openai/openai-go) SDK.

The dispatcher is provider-neutral at runtime. Configure a base URL, model, and
API-key environment variable for OpenAI, Azure-compatible gateways, Ollama,
vLLM, LM Studio, LiteLLM, or another server that implements the relevant
OpenAI-compatible endpoint.

## Capabilities

- `openai.chat` — Chat Completions API; the broadest compatibility surface.
- `openai.responses` — Responses API for providers that implement it.
- `openai.embeddings` — Embeddings API.
- `openai.models.list` — Models API.

Each operation is an independent manifest capability:

```json
{
  "capabilities": [
    {
      "name": "openai.chat",
      "settings": {
        "base_url": "https://llm.example.com/v1",
        "api_key_env": "LLM_API_KEY",
        "default_model": "provider/model",
        "allowed_models": ["provider/model"],
        "require_approval": false
      }
    },
    {
      "name": "openai.models.list",
      "settings": {
        "base_url": "https://llm.example.com/v1",
        "api_key_env": "LLM_API_KEY"
      }
    }
  ]
}
```

Generation and embedding capabilities require human approval by default because
they send data to a provider and may incur cost. `openai.models.list` does not.
Set `require_approval` explicitly on each capability to override the default.

## Settings

- `base_url`: API root including `/v1`; defaults to OpenAI.
- `api_key_env`: environment variable containing the API key; defaults to
  `OPENAI_API_KEY`. The secret itself is never stored in the manifest.
- `default_model`: model used when a request omits `model`.
- `allowed_models`: exact model IDs permitted for this capability; empty allows
  all models.
- `organization`: optional OpenAI organization.
- `project`: optional OpenAI project.
- `timeout`: complete request timeout as a Go duration, such as `2m`.
- `max_retries`: SDK retry count; defaults to 2.
- `allow_insecure_http`: permits plain HTTP only for loopback hosts.
- `headers_from_env`: maps HTTP header names to environment variables. This
  supports compatible gateways without embedding credentials in manifests.
- `require_approval`: per-capability approval override.

All capabilities configured into one Aurora dispatcher instance must use the
same connection settings. Model allowlists and approval requirements remain
per capability.

## Requests

`openai.chat` accepts standard text messages and common generation controls:

```json
{
  "model": "provider/model",
  "messages": [
    {"role": "system", "content": "Answer concisely."},
    {"role": "user", "content": "Explain Raft."}
  ],
  "temperature": 0.2,
  "max_tokens": 500,
  "response_format": {"type": "json_object"}
}
```

`openai.responses` accepts `input` as a string or provider-compatible JSON,
while `openai.embeddings` accepts a string or string array. Responses preserve
the provider’s complete JSON payload so callers do not lose provider-specific
fields.

## Integration

Register the dispatcher with Aurora’s capability registry:

```go
registry.New(
    openaillm.Registration{},
)
```

This repository uses sibling `replace` directives for local Aurora development.
Replace them with published module versions when consuming it independently.

