package keyring

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"regexp"
	"sort"
	"strings"
)

// Finding is a detected credential with its plaintext already discarded.
// Only the provider, the SHA-256 fingerprint, and a masked hint survive.
// SourceEnvVar is set only for environment-detected keys.
type Finding struct {
	Provider     string
	Fingerprint  string // sha256:<16 hex chars>
	Hint         string // masked display form, e.g. "sk-ant-a…8f2c"
	SourceEnvVar string // e.g. ANTHROPIC_API_KEY; empty for file scans
}

// Fingerprint derives the manifest's key representation from a raw key:
// "sha256:<first 16 hex of sha256>" plus a masked hint. Callers must drop
// the raw key immediately after this call.
func Fingerprint(key string) (fingerprint, hint string) {
	sum := sha256.Sum256([]byte(key))
	fingerprint = "sha256:" + hex.EncodeToString(sum[:])[:16]
	if len(key) >= 16 {
		hint = key[:8] + "…" + key[len(key)-4:]
	} else {
		hint = "…"
	}
	return fingerprint, hint
}

// fallbackKeyPattern is the openai-compatible shape: any OpenAI-style key no
// specific vendor claimed. Entropy-gated in Detect.
var fallbackKeyPattern = regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`)

// Detect scans text (file content) for credential shapes and returns one
// Finding per distinct key, provider-attributed, sorted by fingerprint.
// Specific vendor shapes win over the openai-compatible fallback; the
// fallback additionally requires the key tail to look random (entropy gate).
func Detect(content string) []Finding {
	type hit struct {
		provider, key string
	}
	var hits []hit
	seenKey := map[string]bool{}

	for _, p := range All() {
		if p.ID == "openai-compatible" {
			continue
		}
		for _, m := range p.KeyPattern.FindAllString(content, -1) {
			if seenKey[m] {
				continue
			}
			seenKey[m] = true
			hits = append(hits, hit{p.ID, m})
		}
	}
	// Fallback pass: openai-shaped keys no specific vendor claimed.
	for _, m := range fallbackKeyPattern.FindAllString(content, -1) {
		if seenKey[m] {
			continue
		}
		if shannonBitsPerChar(m[3:]) < 3.0 {
			continue
		}
		seenKey[m] = true
		hits = append(hits, hit{"openai-compatible", m})
	}

	out := make([]Finding, 0, len(hits))
	for _, h := range hits {
		fp, hint := Fingerprint(h.key)
		out = append(out, Finding{Provider: h.provider, Fingerprint: fp, Hint: hint})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Fingerprint < out[j].Fingerprint })
	return out
}

// DetectEnv scans KEY=VALUE environment entries the same way.
func DetectEnv(env []string) []Finding {
	var out []Finding
	seen := map[string]bool{}
	for _, kv := range env {
		name, value, found := strings.Cut(kv, "=")
		if !found || value == "" {
			continue
		}
		for _, f := range Detect(value) {
			if seen[f.Fingerprint] {
				continue
			}
			seen[f.Fingerprint] = true
			f.SourceEnvVar = name
			out = append(out, f)
		}
	}
	return out
}

// shannonBitsPerChar measures the randomness of s. Low entropy (repeated
// patterns, padding) keeps placeholder or dummy strings out of the
// openai-compatible bucket; vendor-prefixed keys skip this gate entirely.
func shannonBitsPerChar(s string) float64 {
	if s == "" {
		return 0
	}
	freq := map[byte]int{}
	for i := 0; i < len(s); i++ {
		freq[s[i]]++
	}
	var bits float64
	for _, n := range freq {
		p := float64(n) / float64(len(s))
		bits -= p * math.Log2(p)
	}
	return bits
}
