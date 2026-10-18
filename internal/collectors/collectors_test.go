package collectors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/SuperMarioYL/agentmanifest/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gojsonschema "github.com/xeipuuv/gojsonschema"
)

// fixtureRoot is the shipped offline fixture: a fixture HOME plus a fixture
// project (opencode.json, .mcp.json, project skills). All keys in it are
// fake, vendor-shaped placeholders.
var fixtureRoot = mustFixtureRoot()

func mustFixtureRoot() string {
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures"))
	if err != nil {
		panic(err)
	}
	return root
}

func scanFixtures(t *testing.T) manifest.Manifest {
	t.Helper()
	m, err := Run(Options{
		Home:    filepath.Join(fixtureRoot, "home"),
		Project: fixtureRoot,
		Env: []string{
			"GROQ_API_KEY=gsk_FAKE0123456789abcdefghijklmnop",
			"PATH=/usr/bin:/bin",
		},
	})
	require.NoError(t, err)
	require.NoError(t, m.Validate(), "fixture scan must produce a schema-valid manifest")
	return m
}

func findTool(t *testing.T, m manifest.Manifest, name, kind string) manifest.Tool {
	t.Helper()
	for _, tool := range m.Tools {
		if tool.Name == name && tool.Kind == kind {
			return tool
		}
	}
	t.Fatalf("tool %q (kind %s) not found in manifest", name, kind)
	return manifest.Tool{}
}

func TestFixtureScanToolInventory(t *testing.T) {
	m := scanFixtures(t)

	// CLI tools: opencode, claude-code, codex + the host-environment entry.
	for _, want := range [][2]string{
		{"opencode", manifest.KindCLI},
		{"claude-code", manifest.KindCLI},
		{"codex", manifest.KindCLI},
		{"host-environment", manifest.KindCLI},
		{"opencode:fetch", manifest.KindMCPServer},
		{"opencode:context7", manifest.KindMCPServer},
		{"claude-code:docs", manifest.KindMCPServer},
		{"claude-code:brave-search", manifest.KindMCPServer},
		{"release-notes", manifest.KindSkill},
		{"web-reporter", manifest.KindSkill},
	} {
		findTool(t, m, want[0], want[1])
	}
	assert.Len(t, m.Tools, 10)
}

func TestFixtureScanCredentialAttribution(t *testing.T) {
	m := scanFixtures(t)

	opencode := findTool(t, m, "opencode", manifest.KindCLI)
	providers := credentialProviders(opencode)
	assert.ElementsMatch(t, []string{"anthropic", "openrouter", "openai"}, providers,
		"opencode: project config (anthropic), global config (openrouter), data dir auth.json (openai)")
	for _, c := range opencode.Credentials {
		assert.NotEmpty(t, c.SourceFile)
	}

	claude := findTool(t, m, "claude-code", manifest.KindCLI)
	assert.Equal(t, []string{"anthropic"}, credentialProviders(claude),
		"claude-code: settings.json env block")

	codex := findTool(t, m, "codex", manifest.KindCLI)
	assert.Equal(t, []string{"openai"}, credentialProviders(codex),
		"codex: auth.json stored key")

	brave := findTool(t, m, "claude-code:brave-search", manifest.KindMCPServer)
	assert.Equal(t, []string{"github"}, credentialProviders(brave),
		"mcp server env from .mcp.json")
	for _, c := range brave.Credentials {
		assert.Contains(t, c.SourceFile, ".mcp.json")
	}

	envTool := findTool(t, m, "host-environment", manifest.KindCLI)
	assert.Equal(t, []string{"groq"}, credentialProviders(envTool))
	for _, c := range envTool.Credentials {
		assert.Equal(t, "env:GROQ_API_KEY", c.SourceFile)
	}
}

func credentialProviders(t manifest.Tool) []string {
	out := make([]string, 0, len(t.Credentials))
	for _, c := range t.Credentials {
		out = append(out, c.Provider)
	}
	sort.Strings(out)
	return out
}

func TestFixtureScanReachabilityDerivedStatically(t *testing.T) {
	m := scanFixtures(t)

	opencode := findTool(t, m, "opencode", manifest.KindCLI)
	require.Len(t, opencode.Reachability, len(opencode.Credentials))
	for _, r := range opencode.Reachability {
		switch r.Provider {
		case "anthropic":
			assert.Equal(t, []string{"https://api.anthropic.com/v1"}, r.APIEndpoints)
			assert.Equal(t, "console.anthropic.com", r.AuthDomain)
		case "openrouter":
			assert.Equal(t, []string{"https://openrouter.ai/api/v1"}, r.APIEndpoints)
		case "openai":
			assert.Equal(t, []string{"https://api.openai.com/v1"}, r.APIEndpoints)
		default:
			t.Errorf("unexpected provider %q", r.Provider)
		}
	}

	// host-environment holds a groq key → reachable, with content destinations.
	envTool := findTool(t, m, "host-environment", manifest.KindCLI)
	require.Len(t, envTool.Reachability, 1)
	assert.Equal(t, "groq", envTool.Reachability[0].Provider)
	var dests []string
	for _, cs := range envTool.ContentSurface {
		dests = append(dests, cs.Destinations...)
	}
	assert.Contains(t, dests, "api.groq.com")
}

func TestFixtureScanEgressLayers(t *testing.T) {
	m := scanFixtures(t)

	codex := findTool(t, m, "codex", manifest.KindCLI)
	var codexDomains []string
	for _, d := range codex.DeclaredEgress {
		codexDomains = append(codexDomains, d.Domain)
	}
	assert.ElementsMatch(t, []string{"api.openai.com", "relay.internal.example"}, codexDomains,
		"codex config.toml base_url entries")

	// Skill instructions point at their upload endpoints.
	reporter := findTool(t, m, "web-reporter", manifest.KindSkill)
	var reporterDomains []string
	for _, d := range reporter.DeclaredEgress {
		reporterDomains = append(reporterDomains, d.Domain)
	}
	assert.Contains(t, reporterDomains, "reporter.alphadocs.example")

	// Observed egress comes from the fixture log only, filtered to known
	// provider/declared hosts — the npm registry hit must not appear.
	opencode := findTool(t, m, "opencode", manifest.KindCLI)
	var observed []string
	for _, d := range opencode.ObservedEgress {
		observed = append(observed, d.Domain)
		assert.NotEmpty(t, d.EvidenceSource)
	}
	assert.ElementsMatch(t, []string{"api.anthropic.com", "api.openai.com"}, observed)
}

func TestFixtureScanContentSurfaceByKind(t *testing.T) {
	m := scanFixtures(t)

	opencode := findTool(t, m, "opencode", manifest.KindCLI)
	classes := contentClasses(opencode)
	assert.Equal(t, []string{"prompts", "code", "env"}, classes)

	docs := findTool(t, m, "claude-code:docs", manifest.KindMCPServer)
	assert.Equal(t, []string{"prompts", "env"}, contentClasses(docs))

	notes := findTool(t, m, "release-notes", manifest.KindSkill)
	assert.Equal(t, []string{"prompts"}, contentClasses(notes))
	assert.Empty(t, notes.Credentials, "release-notes skill holds no keys")
}

func contentClasses(t manifest.Tool) []string {
	out := make([]string, 0, len(t.ContentSurface))
	for _, cs := range t.ContentSurface {
		out = append(out, cs.DataClass)
	}
	return out
}

func TestFixtureScanSerializationNeverCarriesPlaintext(t *testing.T) {
	m := scanFixtures(t)

	// Every fake raw key shipped in the fixtures — none of these strings may
	// survive into the serialized manifest.
	rawKeys := []string{
		"sk-ant-api03-FAKE0000EXAMPLE0000FAKE0000EXAMPLE",
		"sk-or-v1-deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"sk-proj-FAKE0000000000000000EXAMPLE",
		"sk-ant-api03-FAKE1111EXAMPLE1111FAKE1111EXAMPLE",
		"sk-None-FAKE222222222222222EXAMPLE",
		"github_pat_FAKE0123456789EXAMPLE0123456789",
		"hf_FAKE0123456789abcdef0123456789abcdef012345",
		"gsk_FAKE0123456789abcdefghijklmnop",
	}
	yamlBytes, err := m.MarshalYAMLBytes()
	require.NoError(t, err)
	jsonBytes, err := m.MarshalJSONBytes()
	require.NoError(t, err)
	for _, out := range []string{string(yamlBytes), string(jsonBytes)} {
		for _, key := range rawKeys {
			assert.NotContains(t, out, key)
		}
		assert.Contains(t, out, "sha256:")
	}
}

func TestFixtureScanJSONRoundTrip(t *testing.T) {
	m := scanFixtures(t)
	jsonBytes, err := m.MarshalJSONBytes()
	require.NoError(t, err)
	var back manifest.Manifest
	require.NoError(t, json.Unmarshal(jsonBytes, &back))
	require.NoError(t, back.Validate())
	assert.Equal(t, manifest.SchemaVersion, back.SchemaVersion)
	assert.Equal(t, len(m.Tools), len(back.Tools))
}

func TestScanHandlesMissingPathsSilently(t *testing.T) {
	// Empty home and project: no crash, no tools — scan is read-only and
	// tolerant of machines without any agent tooling.
	m, err := Run(Options{
		Home:    t.TempDir(),
		Project: t.TempDir(),
		Env:     []string{"PATH=/usr/bin"},
	})
	require.NoError(t, err)
	require.NoError(t, m.Validate())
	assert.Empty(t, m.Tools)
}

func TestDisplayPathMasksScanScope(t *testing.T) {
	sep := string(os.PathSeparator)
	home := sep + filepath.Join("Users", "dev")
	project := sep + filepath.Join("srv", "demo")
	s := &scanner{opts: Options{Home: home, Project: project}}

	assert.True(t, strings.HasPrefix(
		s.displayPath(filepath.Join(home, ".claude", "settings.json")), "~/.claude/"),
		"home-prefixed paths must mask as ~")
	assert.Equal(t, filepath.Join("<project>", ".mcp.json"),
		s.displayPath(filepath.Join(project, ".mcp.json")),
		"project-rooted paths must mask as <project>/…")
	other := sep + filepath.Join("opt", "thing.json")
	assert.Equal(t, other, s.displayPath(other), "paths outside the scan scope pass through")

	// Demo-style layout: the scan home lives under the project directory;
	// home paths must still win the mask.
	nested := &scanner{opts: Options{Home: filepath.Join(project, "home"), Project: project}}
	assert.True(t, strings.HasPrefix(
		nested.displayPath(filepath.Join(project, "home", ".claude", "settings.json")), "~/.claude/"),
		"home mask must take precedence over the project mask")
}

func TestOpenAICompatibleKeyEmitsSchemaValidJSON(t *testing.T) {
	// A bare, entropy-passing sk-… key that matches no specific vendor maps
	// to the openai-compatible fallback, whose endpoints are unknown from the
	// key alone. The reachability row must still serialize api_endpoints as
	// [] (not null) so the emitted JSON conforms to
	// schemas/exposure-manifest.v0.json, which requires an array.
	m, err := Run(Options{
		Home:    filepath.Join(fixtureRoot, "home"),
		Project: fixtureRoot,
		Env: []string{
			// Fake, vendor-shaped placeholder; entropy passes the fallback gate.
			"DEEPSEEK_API_KEY=sk-Qk7vX2mN9pR4tW8zY3aB6cD1eF0gH5jL",
			"PATH=/usr/bin:/bin",
		},
	})
	require.NoError(t, err)

	envTool := findTool(t, m, "host-environment", manifest.KindCLI)
	require.Len(t, envTool.Credentials, 1)
	require.Len(t, envTool.Reachability, 1)
	row := envTool.Reachability[0]
	assert.Equal(t, "openai-compatible", row.Provider)
	assert.NotNil(t, row.APIEndpoints, "api_endpoints must not be nil in the emitted JSON")
	assert.Empty(t, row.APIEndpoints, "unknown endpoints serialize as an empty array")

	jsonBytes, err := m.MarshalJSONBytes()
	require.NoError(t, err)
	schemaBytes, err := os.ReadFile(filepath.Join("..", "..", "schemas", "exposure-manifest.v0.json"))
	require.NoError(t, err)
	result, err := gojsonschema.Validate(
		gojsonschema.NewBytesLoader(schemaBytes),
		gojsonschema.NewBytesLoader(jsonBytes),
	)
	require.NoError(t, err)
	assert.True(t, result.Valid(),
		"emitted JSON must validate against schemas/exposure-manifest.v0.json: %v", result.Errors())
}
