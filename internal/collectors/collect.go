// Package collectors reads the local machine's known agent CLI / skill / MCP
// configuration paths and builds an ExposureManifest. Collectors are strictly
// read-only: no scanned binary is executed, no network packet is sent, and no
// plaintext credential is kept after fingerprinting.
package collectors

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/SuperMarioYL/agentmanifest/internal/keyring"
	"github.com/SuperMarioYL/agentmanifest/internal/manifest"
)

const (
	// maxFileBytes caps any single scanned file so a huge state file cannot
	// dominate a 60-second scan budget.
	maxFileBytes = 1 << 20
	// maxSkillFileBytes caps files inside skill directories (instructions, not data).
	maxSkillFileBytes = 1 << 18
	// maxFilesPerDir caps directory walks.
	maxFilesPerDir = 2000
	// maxReadChunk is the sniff window for the binary-content check.
	maxReadChunk = 512
)

// Options selects what to scan. Home and Project are overridable so the same
// code runs against fixtures in CI (offline, deterministic).
type Options struct {
	// Home is the user home holding global agent configs (~/.claude, ~/.codex, ...).
	Home string
	// Project is the working project directory (opencode.json, .mcp.json, project skills).
	Project string
	// Env is the environment block to scan (defaults to os.Environ()).
	Env []string
}

// Run produces a validated-ready manifest for the given scan scope.
func Run(opts Options) (manifest.Manifest, error) {
	if opts.Home == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return manifest.Manifest{}, err
		}
		opts.Home = home
	}
	if opts.Project == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return manifest.Manifest{}, err
		}
		opts.Project = cwd
	}
	if opts.Env == nil {
		opts.Env = os.Environ()
	}

	host, err := os.Hostname()
	if err != nil {
		host = "unknown-host"
	}

	m := manifest.New(HostFingerprint(host), time.Now().UTC())
	s := &scanner{opts: opts}
	s.collectOpenCode()
	s.collectClaude()
	s.collectCodex()
	s.collectSkills()
	s.collectEnv()

	m.Tools = s.tools
	for i := range m.Tools {
		finalize(&m.Tools[i])
	}
	sortTools(m.Tools)
	return m, nil
}

type scanner struct {
	opts  Options
	tools []manifest.Tool
}

// appendTool attaches a fully collected tool entry to the manifest. Tool
// values are built locally by each collector and appended exactly once, so
// no entry is mutated through a pointer that a later append could orphan.
func (s *scanner) appendTool(t manifest.Tool) {
	s.tools = append(s.tools, t)
}

// addCredentials fingerprint-detects every credential shape in content and
// attaches the findings to t. The raw text is dropped on return.
func addCredentials(t *manifest.Tool, sourceFile, content string) {
	for _, f := range keyring.Detect(content) {
		if hasCredential(t, f.Fingerprint) {
			continue
		}
		t.Credentials = append(t.Credentials, manifest.Credential{
			Provider:       f.Provider,
			KeyFingerprint: f.Fingerprint,
			KeyHint:        f.Hint,
			SourceFile:     sourceFile,
		})
	}
}

func hasCredential(t *manifest.Tool, fingerprint string) bool {
	for _, c := range t.Credentials {
		if c.KeyFingerprint == fingerprint {
			return true
		}
	}
	return false
}

var urlPattern = regexp.MustCompile(`https?://[^\s"'` + "`" + `<>()\[\]{}]+`)

// addDeclaredEgress extracts hostnames from URLs found in config or skill
// text and records them as declared egress.
func addDeclaredEgress(t *manifest.Tool, source, content string) {
	for _, host := range extractHosts(content) {
		if hasDeclared(t, host) {
			continue
		}
		t.DeclaredEgress = append(t.DeclaredEgress, manifest.DeclaredEgress{
			Domain: host,
			Source: source,
		})
	}
}

func hasDeclared(t *manifest.Tool, domain string) bool {
	for _, d := range t.DeclaredEgress {
		if d.Domain == domain {
			return true
		}
	}
	return false
}

// observedHosts is the filter set for observed egress: hosts of known provider
// endpoints, plus everything already declared. Log URLs outside this set are
// mostly documentation links and would be noise in an archived manifest.
func observedHosts() map[string]bool {
	hosts := map[string]bool{}
	for _, p := range keyring.All() {
		for _, ep := range p.APIEndpoints {
			if u, err := url.Parse(ep); err == nil {
				hosts[u.Host] = true
			}
		}
		if p.AuthDomain != "" {
			hosts[p.AuthDomain] = true
		}
	}
	return hosts
}

// addObservedEgress records hostnames found in local logs, restricted to the
// known/declared set. Evidence is the log file itself — no probe was sent.
func addObservedEgress(t *manifest.Tool, evidenceFile, content string, known map[string]bool) {
	for _, host := range extractHosts(content) {
		if !known[host] || hasObserved(t, host) {
			continue
		}
		t.ObservedEgress = append(t.ObservedEgress, manifest.ObservedEgress{
			Domain:         host,
			EvidenceSource: evidenceFile,
		})
	}
}

func hasObserved(t *manifest.Tool, domain string) bool {
	for _, d := range t.ObservedEgress {
		if d.Domain == domain {
			return true
		}
	}
	return false
}

// extractHosts pulls unique hostnames out of every URL-shaped substring.
func extractHosts(content string) []string {
	set := map[string]bool{}
	for _, raw := range urlPattern.FindAllString(content, -1) {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue
		}
		host := strings.ToLower(u.Host)
		// strip credentials in URL if any
		if at := strings.LastIndex(host, "@"); at >= 0 {
			host = host[at+1:]
		}
		if host != "" {
			set[host] = true
		}
	}
	out := make([]string, 0, len(set))
	for h := range set {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// finalize derives the layers that are computed rather than collected:
// reachability (key shape → vendor → endpoint mapping) and content surface
// (best-effort static statement by tool kind).
func finalize(t *manifest.Tool) {
	// Reachability: one row per held credential with a mapped provider.
	for _, c := range t.Credentials {
		if hasReachability(t, c.KeyFingerprint) {
			continue
		}
		p, ok := keyring.Lookup(c.Provider)
		if !ok {
			continue
		}
		endpoints := p.APIEndpoints
		if endpoints == nil {
			// The provider maps (key shape recognized) but no endpoint is
			// known from the key alone — e.g. the openai-compatible fallback.
			// The schema requires api_endpoints to be an array, so emit an
			// empty one rather than null.
			endpoints = []string{}
		}
		t.Reachability = append(t.Reachability, manifest.Reachability{
			KeyFingerprint: c.KeyFingerprint,
			Provider:       c.Provider,
			APIEndpoints:   endpoints,
			AuthDomain:     p.AuthDomain,
		})
	}

	// Content surface destinations: reachable endpoints + declared domains.
	destSet := map[string]bool{}
	var dests []string
	for _, r := range t.Reachability {
		for _, ep := range r.APIEndpoints {
			if u, err := url.Parse(ep); err == nil && u.Host != "" && !destSet[u.Host] {
				destSet[u.Host] = true
				dests = append(dests, u.Host)
			}
		}
	}
	for _, d := range t.DeclaredEgress {
		if !destSet[d.Domain] {
			destSet[d.Domain] = true
			dests = append(dests, d.Domain)
		}
	}
	sort.Strings(dests)

	var classes []string
	switch t.Kind {
	case manifest.KindCLI:
		classes = []string{manifest.DataPrompts, manifest.DataCode, manifest.DataEnv}
	case manifest.KindMCPServer:
		classes = []string{manifest.DataPrompts, manifest.DataEnv}
	case manifest.KindSkill:
		classes = []string{manifest.DataPrompts}
	}
	for _, class := range classes {
		t.ContentSurface = append(t.ContentSurface, manifest.ContentSurface{
			DataClass:    class,
			Destinations: dests,
		})
	}
}

func hasReachability(t *manifest.Tool, fingerprint string) bool {
	for _, r := range t.Reachability {
		if r.KeyFingerprint == fingerprint {
			return true
		}
	}
	return false
}

// dedupeTools collapses entries created for the same tool identity.
func sortTools(tools []manifest.Tool) {
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Kind != tools[j].Kind {
			return tools[i].Kind < tools[j].Kind
		}
		return tools[i].Name < tools[j].Name
	})
}

// HostFingerprint reduces the hostname to a stable, non-reversible tag so the
// archived manifest can be tied to a machine without publishing its name.
func HostFingerprint(hostname string) string {
	sum := sha256.Sum256([]byte(hostname))
	return "host_" + hex.EncodeToString(sum[:])[:16]
}

// displayPath renders paths inside the scan scope with placeholders so
// archived manifests do not leak the local username or the absolute location
// of the scanned project: home-prefixed paths become ~/… and project-rooted
// paths become <project>/…, exactly mirroring the labels collectSkills uses.
// The home check runs first: a fixture-style project may sit under the scan
// home, and those paths must keep masking as ~.
func (s *scanner) displayPath(path string) string {
	if s.opts.Home != "" && strings.HasPrefix(path, s.opts.Home+string(filepath.Separator)) {
		return "~" + path[len(s.opts.Home):]
	}
	if s.opts.Project != "" && strings.HasPrefix(path, s.opts.Project+string(filepath.Separator)) {
		return "<project>" + path[len(s.opts.Project):]
	}
	return path
}

// readCap reads a file whose size is already capped, returning false when the
// file is unreadable or binary-looking.
func readCap(path string, max int64) (string, bool) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() || st.Size() > max {
		return "", false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	content := string(raw)
	if strings.ContainsRune(content[:min(len(content), maxReadChunk)], 0) {
		return "", false // binary
	}
	return content, true
}
