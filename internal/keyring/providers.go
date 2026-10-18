// Package keyring maps credential shapes → vendor → API endpoints. Detection
// is static only: vendor prefix + regex + entropy, plus a vendor prefix map
// for reachability. It never sends a probe packet and never returns plaintext.
package keyring

import "regexp"

// Provider describes one credential vendor as far as a static inventory needs:
// which key shapes belong to it, which API base the key can reach, and which
// domain the operator signs in on. Reachability is derived from this mapping,
// never measured.
type Provider struct {
	ID           string
	KeyPattern   *regexp.Regexp
	APIEndpoints []string
	AuthDomain   string
	// EnvVars lists canonical environment variable names, used for labelling.
	EnvVars []string
}

// providers is ordered: the most specific shapes first. The openai-compatible
// fallback must stay last — it claims any leftover "sk-" key.
var providers = []Provider{
	{
		ID:           "anthropic",
		KeyPattern:   regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`),
		APIEndpoints: []string{"https://api.anthropic.com/v1"},
		AuthDomain:   "console.anthropic.com",
		EnvVars:      []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"},
	},
	{
		ID: "openai",
		// sk-proj-… / sk-None-… / sk-svcacct-… — the project/service forms of
		// OpenAI keys; the bare "sk-…" shape falls through to openai-compatible.
		KeyPattern:   regexp.MustCompile(`sk-(?:proj|None|svcacct)-[A-Za-z0-9_-]{20,}`),
		APIEndpoints: []string{"https://api.openai.com/v1"},
		AuthDomain:   "platform.openai.com",
		EnvVars:      []string{"OPENAI_API_KEY"},
	},
	{
		ID:           "openrouter",
		KeyPattern:   regexp.MustCompile(`sk-or-v1-[a-f0-9]{32,}`),
		APIEndpoints: []string{"https://openrouter.ai/api/v1"},
		AuthDomain:   "openrouter.ai",
		EnvVars:      []string{"OPENROUTER_API_KEY"},
	},
	{
		ID: "google",
		// AIza… keys authenticate Gemini API calls (and other Google APIs).
		KeyPattern:   regexp.MustCompile(`AIza[0-9A-Za-z_-]{30,}`),
		APIEndpoints: []string{"https://generativelanguage.googleapis.com/v1beta"},
		AuthDomain:   "aistudio.google.com",
		EnvVars:      []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"},
	},
	{
		ID:           "groq",
		KeyPattern:   regexp.MustCompile(`gsk_[A-Za-z0-9]{20,}`),
		APIEndpoints: []string{"https://api.groq.com/openai/v1"},
		AuthDomain:   "console.groq.com",
		EnvVars:      []string{"GROQ_API_KEY"},
	},
	{
		ID:           "xai",
		KeyPattern:   regexp.MustCompile(`xai-[A-Za-z0-9]{20,}`),
		APIEndpoints: []string{"https://api.x.ai/v1"},
		AuthDomain:   "console.x.ai",
		EnvVars:      []string{"XAI_API_KEY"},
	},
	{
		ID:           "github",
		KeyPattern:   regexp.MustCompile(`(?:ghp|gho|ghu|ghs)_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{20,}`),
		APIEndpoints: []string{"https://api.github.com"},
		AuthDomain:   "github.com",
		EnvVars:      []string{"GITHUB_TOKEN", "GITHUB_PERSONAL_ACCESS_TOKEN"},
	},
	{
		ID:           "gitlab",
		KeyPattern:   regexp.MustCompile(`glpat-[A-Za-z0-9_-]{15,}`),
		APIEndpoints: []string{"https://gitlab.com/api/v4"},
		AuthDomain:   "gitlab.com",
		EnvVars:      []string{"GITLAB_TOKEN"},
	},
	{
		ID:           "huggingface",
		KeyPattern:   regexp.MustCompile(`hf_[A-Za-z0-9]{30,}`),
		APIEndpoints: []string{"https://router.huggingface.co/v1"},
		AuthDomain:   "huggingface.co",
		EnvVars:      []string{"HF_TOKEN", "HUGGING_FACE_HUB_TOKEN"},
	},
	{
		ID:           "slack",
		KeyPattern:   regexp.MustCompile(`xox[bpars]-[A-Za-z0-9-]{10,}`),
		APIEndpoints: []string{"https://slack.com/api"},
		AuthDomain:   "slack.com",
		EnvVars:      []string{"SLACK_BOT_TOKEN", "SLACK_TOKEN"},
	},
	{
		ID:           "nvidia",
		KeyPattern:   regexp.MustCompile(`nvapi-[A-Za-z0-9_-]{20,}`),
		APIEndpoints: []string{"https://integrate.api.nvidia.com/v1"},
		AuthDomain:   "build.nvidia.com",
		EnvVars:      []string{"NVIDIA_API_KEY"},
	},
	{
		ID: "openai-compatible",
		// Any OpenAI-shaped key that did not match a specific vendor above:
		// DeepSeek, Moonshot, local relays, and other OpenAI-protocol vendors.
		// The endpoints are unknown from the key alone, so reachability stays
		// empty — entropy gating keeps random "sk-" strings from being claimed.
		KeyPattern:   regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
		APIEndpoints: nil,
		AuthDomain:   "",
		EnvVars:      nil,
	},
}

// All returns the provider registry in detection order.
func All() []Provider { return providers }

// Lookup finds a provider by ID.
func Lookup(id string) (Provider, bool) {
	for _, p := range providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}
