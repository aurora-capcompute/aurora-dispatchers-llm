package openaillm

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultBaseURL         = "https://api.openai.com/v1"
	defaultAPIKeyEnv       = "OPENAI_API_KEY"
	defaultTimeout         = 2 * time.Minute
	defaultMaxRequestBytes = 1 << 20
)

var (
	envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	headerPattern  = regexp.MustCompile(`^[!#$%&'*+\-.^_` + "`" + `|~0-9A-Za-z]+$`)
)

type Settings struct {
	BaseURL           string            `json:"base_url,omitempty"`
	APIKeyEnv         string            `json:"api_key_env,omitempty"`
	APIKeyOptional    bool              `json:"api_key_optional,omitempty"`
	DefaultModel      string            `json:"default_model,omitempty"`
	AllowedModels     []string          `json:"allowed_models,omitempty"`
	Organization      string            `json:"organization,omitempty"`
	Project           string            `json:"project,omitempty"`
	Timeout           string            `json:"timeout,omitempty"`
	MaxRetries        *int              `json:"max_retries,omitempty"`
	MaxRequestBytes   int               `json:"max_request_bytes,omitempty"`
	AllowInsecureHTTP bool              `json:"allow_insecure_http,omitempty"`
	HeadersFromEnv    map[string]string `json:"headers_from_env,omitempty"`
	RequireApproval   *bool             `json:"require_approval,omitempty"`
}

type normalizedSettings struct {
	Settings
	timeout time.Duration
}

func normalizeSettings(settings Settings) (normalizedSettings, error) {
	settings.BaseURL = strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if settings.BaseURL == "" {
		settings.BaseURL = defaultBaseURL
	}
	if err := validateBaseURL(settings.BaseURL, settings.AllowInsecureHTTP); err != nil {
		return normalizedSettings{}, err
	}

	settings.APIKeyEnv = strings.TrimSpace(settings.APIKeyEnv)
	if settings.APIKeyEnv == "" {
		settings.APIKeyEnv = defaultAPIKeyEnv
	}
	if !envNamePattern.MatchString(settings.APIKeyEnv) {
		return normalizedSettings{}, fmt.Errorf("invalid api_key_env %q", settings.APIKeyEnv)
	}

	settings.DefaultModel = strings.TrimSpace(settings.DefaultModel)
	settings.Organization = strings.TrimSpace(settings.Organization)
	settings.Project = strings.TrimSpace(settings.Project)
	settings.AllowedModels = cleanList(settings.AllowedModels)

	timeout := defaultTimeout
	if strings.TrimSpace(settings.Timeout) != "" {
		parsed, err := time.ParseDuration(settings.Timeout)
		if err != nil || parsed <= 0 {
			return normalizedSettings{}, fmt.Errorf("invalid timeout %q", settings.Timeout)
		}
		timeout = parsed
		settings.Timeout = parsed.String()
	}
	if settings.MaxRetries != nil && *settings.MaxRetries < 0 {
		return normalizedSettings{}, fmt.Errorf("max_retries must not be negative")
	}
	if settings.MaxRequestBytes < 0 {
		return normalizedSettings{}, fmt.Errorf("max_request_bytes must not be negative")
	}
	if settings.MaxRequestBytes == 0 {
		settings.MaxRequestBytes = defaultMaxRequestBytes
	}

	headers := make(map[string]string, len(settings.HeadersFromEnv))
	for name, envName := range settings.HeadersFromEnv {
		name = http.CanonicalHeaderKey(strings.TrimSpace(name))
		envName = strings.TrimSpace(envName)
		if !headerPattern.MatchString(name) {
			return normalizedSettings{}, fmt.Errorf("invalid header name %q", name)
		}
		switch name {
		case "Authorization", "Content-Length", "Host":
			return normalizedSettings{}, fmt.Errorf("header %q cannot be configured through headers_from_env", name)
		}
		if !envNamePattern.MatchString(envName) {
			return normalizedSettings{}, fmt.Errorf("invalid environment variable %q for header %q", envName, name)
		}
		headers[name] = envName
	}
	settings.HeadersFromEnv = headers

	return normalizedSettings{Settings: settings, timeout: timeout}, nil
}

func validateBaseURL(raw string, allowInsecureHTTP bool) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("base_url must contain only scheme, host, and path")
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if !allowInsecureHTTP {
			return fmt.Errorf("plain HTTP base_url requires allow_insecure_http")
		}
		host := parsed.Hostname()
		if host == "localhost" {
			return nil
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("plain HTTP is only allowed for loopback hosts")
		}
		return nil
	default:
		return fmt.Errorf("base_url scheme must be https")
	}
}

func requiresApproval(name string, settings Settings) bool {
	if settings.RequireApproval != nil {
		return *settings.RequireApproval
	}
	return name != "openai.models.list"
}

type modelPolicy struct {
	allowed map[string]struct{}
}

func newModelPolicy(models []string) modelPolicy {
	allowed := make(map[string]struct{}, len(models))
	for _, model := range models {
		allowed[model] = struct{}{}
	}
	return modelPolicy{allowed: allowed}
}

func (p modelPolicy) check(model string) error {
	if len(p.allowed) == 0 {
		return nil
	}
	if _, ok := p.allowed[model]; !ok {
		return fmt.Errorf("model %q is not allowed (allowed: %s)", model, strings.Join(p.list(), ", "))
	}
	return nil
}

func (p modelPolicy) list() []string {
	models := make([]string, 0, len(p.allowed))
	for model := range p.allowed {
		models = append(models, model)
	}
	sort.Strings(models)
	return models
}

func cleanList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	sort.Strings(cleaned)
	return cleaned
}
