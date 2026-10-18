package manifest

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func validManifest() Manifest {
	m := New("host_0123456789abcdef", time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC))
	m.Tools = []Tool{
		{
			Name:        "opencode",
			Kind:        KindCLI,
			InstallPath: "~/.config/opencode",
			Credentials: []Credential{
				{
					Provider:       "anthropic",
					KeyFingerprint: "sha256:0123456789abcdef",
					KeyHint:        "sk-ant-a…XPLE",
					SourceFile:     "~/.config/opencode/opencode.json",
				},
			},
			Reachability: []Reachability{
				{
					KeyFingerprint: "sha256:0123456789abcdef",
					Provider:       "anthropic",
					APIEndpoints:   []string{"https://api.anthropic.com/v1"},
					AuthDomain:     "console.anthropic.com",
				},
			},
			ContentSurface: []ContentSurface{
				{DataClass: DataPrompts, Destinations: []string{"api.anthropic.com"}},
			},
		},
	}
	return m
}

func TestValidateAcceptsWellFormedManifest(t *testing.T) {
	m := validManifest()
	require.NoError(t, m.Validate())
}

func TestValidateRejectsPlaintextLookingFingerprint(t *testing.T) {
	m := validManifest()
	// The no-plaintext invariant: a credential that is not a fingerprint
	// (i.e. looks like a live key) must fail validation.
	m.Tools[0].Credentials[0].KeyFingerprint = "sk-ant-api03-FAKE0000EXAMPLE"
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a sha256 fingerprint")
}

func TestValidateRejectsBadFields(t *testing.T) {
	tests := map[string]func(*Manifest){
		"schema_version": func(m *Manifest) { m.SchemaVersion = "v9" },
		"host_fingerprint": func(m *Manifest) {
			m.HostFingerprint = "my-laptop"
		},
		"generated_at": func(m *Manifest) { m.GeneratedAt = time.Time{} },
		"kind":         func(m *Manifest) { m.Tools[0].Kind = "plugin" },
		"name":         func(m *Manifest) { m.Tools[0].Name = "" },
		"install_path": func(m *Manifest) { m.Tools[0].InstallPath = "" },
		"data_class": func(m *Manifest) {
			m.Tools[0].ContentSurface[0].DataClass = "photos"
		},
	}
	for name, mutate := range tests {
		m := validManifest()
		mutate(&m)
		assert.Error(t, m.Validate(), "expected %s to be rejected", name)
	}
}

func TestYAMLAndJSONRoundTrip(t *testing.T) {
	m := validManifest()

	yamlBytes, err := m.MarshalYAMLBytes()
	require.NoError(t, err)
	var fromYAML Manifest
	require.NoError(t, yaml.Unmarshal(yamlBytes, &fromYAML))
	require.NoError(t, fromYAML.Validate())
	assert.Equal(t, m.Tools[0].Name, fromYAML.Tools[0].Name)
	assert.Equal(t, m.Tools[0].Credentials[0].KeyFingerprint, fromYAML.Tools[0].Credentials[0].KeyFingerprint)

	jsonBytes, err := m.MarshalJSONBytes()
	require.NoError(t, err)
	var fromJSON Manifest
	require.NoError(t, json.Unmarshal(jsonBytes, &fromJSON))
	require.NoError(t, fromJSON.Validate())
	assert.Equal(t, m.SchemaVersion, fromJSON.SchemaVersion)
	assert.Equal(t, m.Tools[0].Reachability[0].APIEndpoints, fromJSON.Tools[0].Reachability[0].APIEndpoints)
}

func TestSerializedManifestNeverContainsPlaintextKeys(t *testing.T) {
	m := validManifest()
	m.Tools[0].Credentials[0].KeyHint = "sk-ant-a…XPLE"

	yamlBytes, _ := m.MarshalYAMLBytes()
	jsonBytes, _ := m.MarshalJSONBytes()
	for _, out := range []string{string(yamlBytes), string(jsonBytes)} {
		// A live key body would never pass the fingerprint shape check above;
		// this asserts the serialized artifact only carries fingerprints/hints.
		assert.NotContains(t, out, "sk-ant-api03-")
		assert.Contains(t, out, "sha256:0123456789abcdef")
		assert.False(t, strings.Contains(out, "SECRET"))
	}
}
