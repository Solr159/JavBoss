package translation

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

const ProfilesKey = "jav_title_translation_profiles"
const ProviderKey = "jav_title_translation_provider"

type Provider struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	Protocol string `json:"protocol"`
}

var Providers = []Provider{
	{"deepseek", "DeepSeek", "https://api.deepseek.com", "chat"},
	{"openai", "OpenAI", "https://api.openai.com/v1", "responses"},
	{"anthropic", "Claude", "https://api.anthropic.com/v1", "anthropic"},
	{"gemini", "Gemini", "https://generativelanguage.googleapis.com/v1beta", "gemini"},
	{"zhipu", "智谱 / GLM", "https://open.bigmodel.cn/api/paas/v4", "chat"},
	{"kimi", "Kimi", "https://api.moonshot.cn/v1", "chat"},
	{"qwen", "千问 / Qwen", "https://dashscope.aliyuncs.com/compatible-mode/v1", "chat"},
	{"grok", "Grok", "https://api.x.ai/v1", "chat"},
	{"custom", "自定义 / Custom", "", "chat"},
}

type Profile struct {
	APIKey         string         `json:"api_key,omitempty"`
	APIKeySet      bool           `json:"api_key_set,omitempty"`
	BaseURL        string         `json:"base_url"`
	Protocol       string         `json:"protocol"`
	Model          string         `json:"model"`
	Temperature    *float64       `json:"temperature"`
	Thinking       bool           `json:"thinking"`
	Effort         string         `json:"effort"`
	Reasoning      string         `json:"reasoning,omitempty"`
	Request        map[string]any `json:"request,omitempty"`
	FastParams     map[string]any `json:"fast_params,omitempty"`
	ThinkingParams map[string]any `json:"thinking_params,omitempty"`
}
type Capabilities struct {
	Reasoning               string   `json:"reasoning"` // none, toggle, always, unknown
	Temperature             bool     `json:"temperature"`
	TemperatureWithThinking bool     `json:"temperature_with_thinking"`
	TemperatureMax          float64  `json:"temperature_max"`
	Efforts                 []string `json:"efforts"`
	Style                   string   `json:"-"`
}

func LookupProvider(id string) (Provider, bool) {
	for _, p := range Providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}
func ActiveProvider(cfg map[string]string) string {
	if cfg[ProviderKey] == "" {
		return "deepseek"
	}
	return cfg[ProviderKey]
}
func LoadProfiles(cfg map[string]string) map[string]Profile {
	profiles := map[string]Profile{}
	_ = json.Unmarshal([]byte(cfg[ProfilesKey]), &profiles)
	if profiles == nil {
		profiles = map[string]Profile{}
	}
	return profiles
}
func ActiveProfile(cfg map[string]string) Profile { return LoadProfiles(cfg)[ActiveProvider(cfg)] }
func (p Profile) Endpoint(provider string) string {
	if p.BaseURL != "" {
		return strings.TrimRight(p.BaseURL, "/")
	}
	d, _ := LookupProvider(provider)
	return d.BaseURL
}
func (p Profile) WireProtocol(provider string) string {
	if provider == "custom" && p.Protocol != "" {
		return p.Protocol
	}
	d, _ := LookupProvider(provider)
	return d.Protocol
}
func (p Profile) Validate(provider string) error {
	if _, ok := LookupProvider(provider); !ok {
		return fmt.Errorf("unknown translation provider")
	}
	if len(p.APIKey) > 2048 || strings.ContainsAny(p.APIKey, "\r\n") || len(p.Model) > 200 {
		return fmt.Errorf("invalid translation credentials or model")
	}
	if endpoint := p.Endpoint(provider); endpoint != "" {
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("invalid translation API base URL")
		}
	}
	if !slices.Contains([]string{"chat", "responses", "anthropic", "gemini"}, p.WireProtocol(provider)) {
		return fmt.Errorf("invalid translation protocol")
	}
	if p.Temperature != nil && (*p.Temperature < 0 || *p.Temperature > 2) {
		return fmt.Errorf("invalid translation temperature")
	}
	if len(p.Effort) > 32 || !slices.Contains([]string{"", "none", "toggle", "always"}, p.Reasoning) {
		return fmt.Errorf("invalid reasoning settings")
	}
	if provider != "custom" && (len(p.Request) > 0 || len(p.FastParams) > 0 || len(p.ThinkingParams) > 0 || p.Reasoning != "") {
		return fmt.Errorf("custom parameters require the custom provider")
	}
	for _, params := range []map[string]any{p.Request, p.FastParams, p.ThinkingParams} {
		encoded, err := json.Marshal(params)
		if err != nil || len(encoded) > 16384 {
			return fmt.Errorf("custom parameters are too large")
		}
	}
	return nil
}

// MergeProfiles preserves keys omitted by the browser, including while switching providers.
func MergeProfiles(cfg map[string]string, patch map[string]json.RawMessage) (string, error) {
	profiles := LoadProfiles(cfg)
	for id, raw := range patch {
		p := profiles[id]
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return "", fmt.Errorf("invalid translation profile")
		}
		// JSON request objects replace prior objects; removed fields must stay removed.
		if _, ok := fields["request"]; ok {
			p.Request = nil
		}
		if _, ok := fields["fast_params"]; ok {
			p.FastParams = nil
		}
		if _, ok := fields["thinking_params"]; ok {
			p.ThinkingParams = nil
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", fmt.Errorf("invalid translation profile")
		}
		p.APIKey = strings.TrimSpace(p.APIKey)
		p.BaseURL = strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
		p.Model = strings.TrimSpace(p.Model)
		p.APIKeySet = false
		if err := p.Validate(id); err != nil {
			return "", err
		}
		profiles[id] = p
	}
	encoded, err := json.Marshal(profiles)
	return string(encoded), err
}
func PublicConfig(cfg map[string]string) {
	provider := ActiveProvider(cfg)
	profiles := LoadProfiles(cfg)
	p := profiles[provider]
	for id, profile := range profiles {
		profile.APIKeySet = profile.APIKey != ""
		profile.APIKey = ""
		profiles[id] = profile
	}
	encoded, _ := json.Marshal(profiles)
	cfg[ProfilesKey] = string(encoded)
	cfg[ProviderKey] = provider
	catalog, _ := json.Marshal(Providers)
	cfg["jav_title_translation_catalog"] = string(catalog)
	caps, _ := json.Marshal(ModelCapabilities(provider, p))
	cfg["jav_title_translation_capabilities"] = string(caps)
	// Filter obsolete persisted fields, especially credentials, from public responses.
	delete(cfg, "jav_title_translation_api_key")
	delete(cfg, "jav_title_translation_api_key_set")
	delete(cfg, "jav_title_translation_model")
	delete(cfg, "jav_title_translation_thinking")
}

// Model discovery rarely includes complete parameter support. Unknown families remain explicit.
// Provider references for these mappings:
// https://api-docs.deepseek.com/guides/thinking_mode/
// https://developers.openai.com/api/docs/guides/reasoning
// https://platform.claude.com/docs/en/build-with-claude/thinking-troubleshooting
// https://ai.google.dev/gemini-api/docs/thinking
// https://docs.z.ai/guides/capabilities/thinking-mode
// https://platform.kimi.ai/docs/api/models-overview
// https://help.aliyun.com/en/model-studio/deep-thinking
// https://docs.x.ai/developers/model-capabilities/text/reasoning
func ModelCapabilities(provider string, p Profile) Capabilities {
	m := strings.ToLower(strings.TrimPrefix(p.Model, "models/"))
	c := Capabilities{Reasoning: "unknown", TemperatureMax: 2, Efforts: []string{}}
	set := func(mode, style string, temp, thinkingTemp bool, efforts ...string) {
		c.Reasoning, c.Style, c.Temperature, c.TemperatureWithThinking = mode, style, temp, thinkingTemp
		if len(efforts) > 0 {
			c.Efforts = efforts
		}
	}
	switch provider {
	case "deepseek":
		set("toggle", "thinking", true, false)
		if strings.Contains(m, "v4") || m == "deepseek-flash" {
			c.Efforts = []string{"low", "high", "max"}
		}
	case "openai":
		switch {
		case strings.HasPrefix(m, "gpt-6"):
			set("toggle", "effort", false, false, "low", "medium", "high", "xhigh", "max")
			if strings.Contains(m, "astra") || strings.Contains(m, "6.1") {
				c.Reasoning = "always"
			}
		case strings.HasPrefix(m, "gpt-5"):
			set("toggle", "effort", true, false, "low", "medium", "high")
			if !strings.HasPrefix(m, "gpt-5.1") && strings.Contains(m, ".") {
				c.Efforts = append(c.Efforts, "xhigh")
			}
			if m == "gpt-5" || strings.HasPrefix(m, "gpt-5-2025") || strings.HasPrefix(m, "gpt-5-mini") || strings.HasPrefix(m, "gpt-5-nano") || strings.Contains(m, "-pro") || strings.Contains(m, "-codex") {
				c.Reasoning = "always"
				c.Temperature = false
			}
		case strings.HasPrefix(m, "o1"), strings.HasPrefix(m, "o3"), strings.HasPrefix(m, "o4"):
			set("always", "effort", false, false, "low", "medium", "high")
		case strings.HasPrefix(m, "gpt-4"), strings.HasPrefix(m, "gpt-3.5"):
			set("none", "", true, true)
		}
	case "anthropic":
		c.TemperatureMax = 1
		switch {
		case strings.Contains(m, "4-6"), strings.Contains(m, "4.6"), strings.Contains(m, "4-7"), strings.Contains(m, "4.7"), strings.Contains(m, "4-8"), strings.Contains(m, "4.8"):
			set("toggle", "adaptive", false, false, "low", "medium", "high")
		case m == "claude-opus-5", m == "claude-sonnet-5", strings.HasPrefix(m, "claude-haiku-5"):
			set("toggle", "adaptive", false, false, "low", "medium", "high")
		case strings.HasPrefix(m, "claude-opus-5"), strings.HasPrefix(m, "claude-sonnet-5"), strings.Contains(m, "mythos"), strings.Contains(m, "fable"):
			set("always", "adaptive", false, false, "low", "medium", "high")
		case strings.Contains(m, "-4"), strings.Contains(m, "3-7"):
			set("toggle", "budget", true, false, "low", "medium", "high")
		case strings.Contains(m, "3-5"), strings.Contains(m, "claude-3-"):
			set("none", "", true, true)
		}
	case "gemini":
		switch {
		case strings.HasPrefix(m, "gemini-3"):
			set("always", "level", false, false, "low", "high")
		case strings.HasPrefix(m, "gemini-2.5"):
			set("toggle", "budget", true, true, "low", "medium", "high")
			if strings.Contains(m, "pro") {
				c.Reasoning = "always"
			}
		case strings.HasPrefix(m, "gemini-2.0"), strings.HasPrefix(m, "gemini-1.5"):
			set("none", "", true, true)
		}
	case "zhipu":
		c.TemperatureMax = 1
		switch {
		case strings.HasPrefix(m, "glm-5.3"):
			set("always", "thinking", true, true, "low", "high", "max")
		case strings.Contains(m, "thinking"), strings.HasPrefix(m, "glm-z1"):
			set("always", "", true, true)
		case strings.HasPrefix(m, "glm-4.5"), strings.HasPrefix(m, "glm-4.6"), strings.HasPrefix(m, "glm-4.7"), strings.HasPrefix(m, "glm-5"):
			set("toggle", "thinking", true, true)
		case strings.HasPrefix(m, "glm-4"):
			set("none", "", true, true)
		}
	case "kimi":
		c.TemperatureMax = 1
		switch {
		case strings.HasPrefix(m, "kimi-k3"):
			set("always", "effort", false, false, "low", "high", "max")
		case strings.Contains(m, "thinking"), strings.HasPrefix(m, "kimi-k2.7"):
			set("always", "", false, false)
		case strings.HasPrefix(m, "kimi-k2.5"), strings.HasPrefix(m, "kimi-k2.6"):
			set("toggle", "thinking", false, false)
		case strings.HasPrefix(m, "kimi-k2"), strings.HasPrefix(m, "moonshot-v1"):
			set("none", "", true, true)
		}
	case "qwen":
		switch {
		case strings.Contains(m, "thinking"), strings.HasPrefix(m, "qwq"), strings.HasPrefix(m, "qvq"):
			set("always", "qwen_budget", true, true, "low", "medium", "high")
		case strings.Contains(m, "instruct"), strings.HasPrefix(m, "qwen2"), strings.Contains(m, "coder"):
			set("none", "", true, true)
		case strings.HasPrefix(m, "qwen3"), strings.HasPrefix(m, "qwen-plus"), strings.HasPrefix(m, "qwen-flash"), strings.HasPrefix(m, "qwen-turbo"):
			set("toggle", "qwen_budget", true, true, "low", "medium", "high")
		case strings.HasPrefix(m, "qwen-max"):
			set("none", "", true, true)
		}
	case "grok":
		switch {
		case strings.Contains(m, "non-reasoning"), strings.HasPrefix(m, "grok-2"), m == "grok-3" || m == "grok-3-latest":
			set("none", "", true, true)
		case strings.HasPrefix(m, "grok-3-mini"):
			set("always", "effort", true, true, "low", "high")
		case strings.HasPrefix(m, "grok-4.5"), strings.HasPrefix(m, "grok-4.6"), strings.HasPrefix(m, "grok-4.7"):
			set("always", "effort", true, true, "low", "medium", "high")
			if !strings.HasPrefix(m, "grok-4.5") {
				c.Efforts = append(c.Efforts, "xhigh")
			}
		case strings.HasPrefix(m, "grok-4"):
			set("always", "", false, false)
		}
	case "custom":
		mode := p.Reasoning
		if mode == "" {
			mode = "none"
		}
		set(mode, "", true, true)
	}
	return c
}
