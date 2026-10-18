// Package manifest defines ExposureManifest (暴露清单) — a versioned, locally
// signable, archivable machine-readable document that describes, for every
// installed agent tool, what it holds, where it can reach, whom its config
// points at, and what user content it may send out.
//
// Design invariant: the manifest itself gets archived, so it must never
// contain plaintext credentials — keys appear only as SHA-256 fingerprints
// with a masked hint. The manifest must not become a new leak source.
package manifest

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// SchemaVersion is the manifest schema version; it matches
// schemas/exposure-manifest.v0.json in the repository.
const SchemaVersion = "v0"

// Tool kinds.
const (
	KindCLI       = "cli"
	KindSkill     = "skill"
	KindMCPServer = "mcp_server"
)

// Content data classes.
const (
	DataPrompts = "prompts"
	DataCode    = "code"
	DataEnv     = "env"
	DataFiles   = "files"
)

var (
	keyFingerprintRe  = regexp.MustCompile(`^sha256:[0-9a-f]{16}$`)
	hostFingerprintRe = regexp.MustCompile(`^host_[0-9a-f]{16}$`)
)

// Manifest is the top-level archive document.
type Manifest struct {
	SchemaVersion   string    `yaml:"schema_version" json:"schema_version"`
	GeneratedAt     time.Time `yaml:"generated_at" json:"generated_at"`
	HostFingerprint string    `yaml:"host_fingerprint" json:"host_fingerprint"`
	// Signature is reserved for m3 (local ed25519 signing); always empty in v0.1.0.
	Signature string `yaml:"signature,omitempty" json:"signature,omitempty"`
	Tools     []Tool `yaml:"tools" json:"tools"`
}

// Tool is one installed agent CLI, third-party skill, or MCP server.
type Tool struct {
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version,omitempty" json:"version,omitempty"`
	Kind        string `yaml:"kind" json:"kind"`
	InstallPath string `yaml:"install_path" json:"install_path"`

	// 持有什么 → 能到哪 → 配置联系谁 → 可能送出什么
	Credentials    []Credential     `yaml:"credentials,omitempty" json:"credentials,omitempty"`
	Reachability   []Reachability   `yaml:"reachability,omitempty" json:"reachability,omitempty"`
	DeclaredEgress []DeclaredEgress `yaml:"declared_egress,omitempty" json:"declared_egress,omitempty"`
	ObservedEgress []ObservedEgress `yaml:"observed_egress,omitempty" json:"observed_egress,omitempty"`
	ContentSurface []ContentSurface `yaml:"content_surface,omitempty" json:"content_surface,omitempty"`
}

// Credential is a key held by the tool. KeyFingerprint is the only
// key representation allowed in a manifest — never plaintext.
type Credential struct {
	Provider       string `yaml:"provider" json:"provider"`
	KeyFingerprint string `yaml:"key_fingerprint" json:"key_fingerprint"`
	// KeyHint is a masked display form (e.g. "sk-ant-…8f2c"); optional.
	KeyHint    string `yaml:"key_hint,omitempty" json:"key_hint,omitempty"`
	SourceFile string `yaml:"source_file" json:"source_file"`
}

// Reachability is derived statically from the key shape → provider → endpoint
// mapping. v0.1 sends no probes; nothing here is the result of a network test.
type Reachability struct {
	KeyFingerprint string   `yaml:"key_fingerprint" json:"key_fingerprint"`
	Provider       string   `yaml:"provider" json:"provider"`
	APIEndpoints   []string `yaml:"api_endpoints" json:"api_endpoints"`
	AuthDomain     string   `yaml:"auth_domain,omitempty" json:"auth_domain,omitempty"`
}

// DeclaredEgress is a domain the tool's own config or skill instructions point at.
type DeclaredEgress struct {
	Domain  string `yaml:"domain" json:"domain"`
	Purpose string `yaml:"purpose,omitempty" json:"purpose,omitempty"`
	// Source names where the declaration was found (skill instruction, MCP config, CLI config).
	Source string `yaml:"source" json:"source"`
}

// ObservedEgress comes from existing local logs and config only — no active probing.
type ObservedEgress struct {
	Domain         string `yaml:"domain" json:"domain"`
	EvidenceSource string `yaml:"evidence_source" json:"evidence_source"`
}

// ContentSurface is a best-effort static statement of which user content
// classes the tool may transmit, and to which destinations.
type ContentSurface struct {
	DataClass    string   `yaml:"data_class" json:"data_class"`
	Destinations []string `yaml:"destinations,omitempty" json:"destinations,omitempty"`
}

// New builds an empty manifest with schema metadata filled in.
func New(hostFingerprint string, now time.Time) Manifest {
	return Manifest{
		SchemaVersion:   SchemaVersion,
		GeneratedAt:     now.UTC(),
		HostFingerprint: hostFingerprint,
	}
}

// Validate checks the structural contract, including the no-plaintext
// invariant: every key representation must look like a fingerprint.
func (m *Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version %q: want %q", m.SchemaVersion, SchemaVersion)
	}
	if m.GeneratedAt.IsZero() {
		return fmt.Errorf("generated_at must be set")
	}
	if !hostFingerprintRe.MatchString(m.HostFingerprint) {
		return fmt.Errorf("host_fingerprint %q: want host_<16 hex>", m.HostFingerprint)
	}
	for i, t := range m.Tools {
		where := fmt.Sprintf("tools[%d]", i)
		if t.Name == "" {
			return fmt.Errorf("%s: name is required", where)
		}
		switch t.Kind {
		case KindCLI, KindSkill, KindMCPServer:
		default:
			return fmt.Errorf("%s (%s): unknown kind %q", where, t.Name, t.Kind)
		}
		if t.InstallPath == "" {
			return fmt.Errorf("%s (%s): install_path is required", where, t.Name)
		}
		for j, c := range t.Credentials {
			if !keyFingerprintRe.MatchString(c.KeyFingerprint) {
				return fmt.Errorf("%s (%s) credential[%d]: key_fingerprint %q is not a sha256 fingerprint",
					where, t.Name, j, c.KeyFingerprint)
			}
			if c.Provider == "" {
				return fmt.Errorf("%s (%s) credential[%d]: provider is required", where, t.Name, j)
			}
			if c.SourceFile == "" {
				return fmt.Errorf("%s (%s) credential[%d]: source_file is required", where, t.Name, j)
			}
		}
		for _, r := range t.Reachability {
			if !keyFingerprintRe.MatchString(r.KeyFingerprint) {
				return fmt.Errorf("%s (%s) reachability: bad key_fingerprint %q", where, t.Name, r.KeyFingerprint)
			}
		}
		for _, d := range t.DeclaredEgress {
			if d.Domain == "" || d.Source == "" {
				return fmt.Errorf("%s (%s) declared_egress: domain and source are required", where, t.Name)
			}
		}
		for _, d := range t.ObservedEgress {
			if d.Domain == "" || d.EvidenceSource == "" {
				return fmt.Errorf("%s (%s) observed_egress: domain and evidence_source are required", where, t.Name)
			}
		}
		for _, c := range t.ContentSurface {
			switch c.DataClass {
			case DataPrompts, DataCode, DataEnv, DataFiles:
			default:
				return fmt.Errorf("%s (%s): unknown data_class %q", where, t.Name, c.DataClass)
			}
		}
	}
	return nil
}

// MarshalJSONBytes renders the manifest as indented JSON.
func (m *Manifest) MarshalJSONBytes() ([]byte, error) {
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// MarshalYAMLBytes renders the manifest as YAML.
func (m *Manifest) MarshalYAMLBytes() ([]byte, error) {
	out, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}
	return out, nil
}
