package keyring

import (
	"regexp"
	"strings"
	"testing"
)

const fingerprintShape = `^sha256:[0-9a-f]{16}$`

func TestDetectAttributesVendorShapes(t *testing.T) {
	content := `
	ANTHROPIC_API_KEY=sk-ant-api03-FAKE0000EXAMPLE0000FAKE0000EXAMPLE
	OPENAI_KEY=sk-proj-FAKE0000000000000000EXAMPLE
	OR_KEY=sk-or-v1-deadbeefdeadbeefdeadbeefdeadbeefdeadbeef
	GH=github_pat_FAKE0123456789EXAMPLE0123456789
	GEMINI=AIzaSyA-FAKE0123456789abcdefEXAMPLE0123
	RELAY=sk-FAKE0123456789abcdefghijklmnopqrst
	`
	got := map[string]string{}
	for _, f := range Detect(content) {
		got[f.Provider] = f.Fingerprint
	}
	want := []string{"anthropic", "openai", "openrouter", "github", "google", "openai-compatible"}
	for _, p := range want {
		if _, ok := got[p]; !ok {
			t.Errorf("provider %q not detected; got %v", p, got)
		}
	}
}

func TestDetectFingerprintShapeAndNoPlaintext(t *testing.T) {
	raw := "sk-ant-api03-FAKE0000EXAMPLE0000FAKE0000EXAMPLE"
	findings := Detect("key: " + raw)
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if !regexp.MustCompile(fingerprintShape).MatchString(f.Fingerprint) {
		t.Errorf("fingerprint %q does not match %s", f.Fingerprint, fingerprintShape)
	}
	// The finding struct only carries provider/fingerprint/hint — the hint
	// must stay masked and must never contain the full raw key.
	if strings.Contains(f.Hint, raw) {
		t.Errorf("hint leaks the raw key: %q", f.Hint)
	}
	if !strings.Contains(f.Hint, "…") {
		t.Errorf("hint %q is not masked", f.Hint)
	}
}

func TestDetectDedupesRepeatedKeys(t *testing.T) {
	raw := "gsk_FAKE0123456789abcdefghijklmnop"
	content := raw + "\n" + raw + "\nsecond file " + raw
	if got := len(Detect(content)); got != 1 {
		t.Errorf("want 1 deduped finding, got %d", got)
	}
}

func TestDetectOpenAICompatibleEntropyGate(t *testing.T) {
	// Low-entropy tail: a padding pattern, not a key — must not be claimed.
	if got := Detect("sk-aaaaaaaaaaaaaaaaaaaaaaaaaaaa"); len(got) != 0 {
		t.Errorf("low-entropy sk- string detected: %+v", got)
	}
	// Random-looking tail: claimed as openai-compatible.
	if got := Detect("sk-9fQ4zXc2vB8nM1kJ7hG5dS3aP0wE6rTy"); len(got) != 1 {
		t.Errorf("random sk- string not detected: %+v", got)
	}
}

func TestDetectEnvLabelsVarName(t *testing.T) {
	env := []string{
		"PATH=/usr/bin:/bin",
		"ANTHROPIC_API_KEY=sk-ant-api03-FAKE0000EXAMPLE0000FAKE0000EXAMPLE",
		"EMPTY=",
	}
	got := DetectEnv(env)
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].SourceEnvVar != "ANTHROPIC_API_KEY" {
		t.Errorf("SourceEnvVar = %q, want ANTHROPIC_API_KEY", got[0].SourceEnvVar)
	}
	if got[0].Provider != "anthropic" {
		t.Errorf("provider = %q, want anthropic", got[0].Provider)
	}
}

func TestProviderRegistryIntegrity(t *testing.T) {
	for _, p := range All() {
		if p.ID == "" || p.KeyPattern == nil {
			t.Errorf("provider %+v missing id or pattern", p)
		}
	}
	if _, ok := Lookup("anthropic"); !ok {
		t.Error("anthropic not found in registry")
	}
	if _, ok := Lookup("no-such-vendor"); ok {
		t.Error("unknown provider unexpectedly found")
	}
}
