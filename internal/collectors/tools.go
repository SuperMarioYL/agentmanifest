package collectors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/SuperMarioYL/agentmanifest/internal/keyring"
	"github.com/SuperMarioYL/agentmanifest/internal/manifest"
)

// collectOpenCode scans opencode sources: the global config, the data
// directory (auth storage + local logs), and the project-level opencode.json.
// opencode config is JSON — $schema points at https://opencode.ai/config.json
// — with a top-level "mcp" mapping whose entries are {type: local, command:
// [...]} (or remote URLs).
func (s *scanner) collectOpenCode() {
	t := manifest.Tool{Name: "opencode", Kind: manifest.KindCLI, InstallPath: "~/.config/opencode"}
	found := false

	globalConfig := filepath.Join(s.opts.Home, ".config", "opencode", "opencode.json")
	if content, ok := readCap(globalConfig, maxFileBytes); ok {
		found = true
		src := displayPath(s.opts.Home, globalConfig)
		s.scanJSONConfig(&t, src, content)
		s.collectOpenCodeMCP(globalConfig, content)
	}

	projectConfig := filepath.Join(s.opts.Project, "opencode.json")
	if content, ok := readCap(projectConfig, maxFileBytes); ok {
		found = true
		src := displayPath(s.opts.Home, projectConfig)
		s.scanJSONConfig(&t, src, content)
		s.collectOpenCodeMCP(projectConfig, content)
	}

	dataDir := filepath.Join(s.opts.Home, ".local", "share", "opencode")
	if dirExists(dataDir) {
		found = true
		known := observedHosts()
		files := 0
		_ = filepath.WalkDir(dataDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			files++
			if files > maxFilesPerDir {
				return filepath.SkipAll
			}
			src := displayPath(s.opts.Home, path)
			switch {
			case strings.HasSuffix(path, ".json"):
				// Auth storage and other JSON state: fingerprint any key shapes.
				if content, ok := readCap(path, maxFileBytes); ok {
					addCredentials(&t, src, content)
				}
			case strings.HasSuffix(path, ".log"):
				// Existing local logs are the only observed-egress evidence:
				// read what is already on disk, never probe.
				if content, ok := readCap(path, maxFileBytes); ok {
					addObservedEgress(&t, src, content, known)
				}
			}
			return nil
		})
	}

	if found {
		s.appendTool(t)
	}
}

// collectOpenCodeMCP creates one mcp_server entry per opencode "mcp" mapping.
func (s *scanner) collectOpenCodeMCP(configPath, content string) {
	var cfg map[string]any
	if json.Unmarshal([]byte(content), &cfg) != nil {
		return
	}
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		return
	}
	for name, raw := range mcp {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		s.mcpToolFromEntry("opencode:"+name, configPath, entry)
	}
}

// collectClaude scans Claude Code sources: global settings.json (env block),
// the global ~/.claude.json mcpServers mapping, and the project .mcp.json.
func (s *scanner) collectClaude() {
	t := manifest.Tool{Name: "claude-code", Kind: manifest.KindCLI, InstallPath: "~/.claude"}
	found := false

	settings := filepath.Join(s.opts.Home, ".claude", "settings.json")
	if content, ok := readCap(settings, maxFileBytes); ok {
		found = true
		addCredentials(&t, displayPath(s.opts.Home, settings), content)
	}

	globalConfig := filepath.Join(s.opts.Home, ".claude.json")
	if content, ok := readCap(globalConfig, maxFileBytes); ok {
		found = true
		s.collectClaudeMCPMap(globalConfig, content)
	}

	projectMCP := filepath.Join(s.opts.Project, ".mcp.json")
	if content, ok := readCap(projectMCP, maxFileBytes); ok {
		found = true
		s.collectClaudeMCPMap(projectMCP, content)
	}

	if found {
		s.appendTool(t)
	}
}

// collectClaudeMCPMap handles {"mcpServers": {name: entry}} files
// (~/.claude.json global servers and project .mcp.json).
func (s *scanner) collectClaudeMCPMap(path, content string) {
	var cfg struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if json.Unmarshal([]byte(content), &cfg) != nil {
		return
	}
	for name, entry := range cfg.MCPServers {
		s.mcpToolFromEntry("claude-code:"+name, path, entry)
	}
}

// mcpToolFromEntry builds an mcp_server tool from one MCP config entry
// ({command/args/env} for local servers, {url} for remote ones).
func (s *scanner) mcpToolFromEntry(name, configPath string, entry map[string]any) {
	installPath := displayPath(s.opts.Home, configPath)
	t := manifest.Tool{Name: name, Kind: manifest.KindMCPServer, InstallPath: installPath}
	encoded, _ := json.Marshal(entry)
	addCredentials(&t, installPath, string(encoded))
	addDeclaredEgress(&t, installPath, string(encoded))
	s.appendTool(t)
}

// collectCodex scans codex sources: config.toml (line-based scan — TOML is
// read with regexes here, which covers base_url entries and inline keys
// without a TOML dependency) and auth.json (stored provider key).
func (s *scanner) collectCodex() {
	t := manifest.Tool{Name: "codex", Kind: manifest.KindCLI, InstallPath: "~/.codex"}
	found := false

	configTOML := filepath.Join(s.opts.Home, ".codex", "config.toml")
	if content, ok := readCap(configTOML, maxFileBytes); ok {
		found = true
		src := displayPath(s.opts.Home, configTOML)
		addCredentials(&t, src, content)
		addDeclaredEgress(&t, src, content)
	}

	authJSON := filepath.Join(s.opts.Home, ".codex", "auth.json")
	if content, ok := readCap(authJSON, maxFileBytes); ok {
		found = true
		addCredentials(&t, displayPath(s.opts.Home, authJSON), content)
	}

	if found {
		s.appendTool(t)
	}
}

// collectEnv fingerprints credential-shaped environment variables. Which CLI
// consumes them is unknowable statically, so they are attributed to a
// host-level entry instead of being pinned onto one tool.
func (s *scanner) collectEnv() {
	findings := keyring.DetectEnv(s.opts.Env)
	if len(findings) == 0 {
		return
	}
	t := manifest.Tool{Name: "host-environment", Kind: manifest.KindCLI, InstallPath: "env"}
	for _, f := range findings {
		if hasCredential(&t, f.Fingerprint) {
			continue
		}
		t.Credentials = append(t.Credentials, manifest.Credential{
			Provider:       f.Provider,
			KeyFingerprint: f.Fingerprint,
			KeyHint:        f.Hint,
			SourceFile:     "env:" + f.SourceEnvVar,
		})
	}
	s.appendTool(t)
}

// scanJSONConfig scans a JSON agent config: every string value is checked for
// credential shapes, and URLs — except the "$schema" pointer, which is a
// document reference rather than egress — are recorded as declared egress.
func (s *scanner) scanJSONConfig(t *manifest.Tool, sourceFile, content string) {
	addCredentials(t, sourceFile, content)
	var cfg any
	if json.Unmarshal([]byte(content), &cfg) != nil {
		return
	}
	var egressText strings.Builder
	walkJSONStrings(cfg, "", func(key, value string) {
		if key == "$schema" {
			return
		}
		egressText.WriteString(value)
		egressText.WriteString("\n")
	})
	addDeclaredEgress(t, sourceFile, egressText.String())
}

// walkJSONStrings visits every string leaf of a decoded JSON document.
func walkJSONStrings(v any, key string, fn func(key, value string)) {
	switch vv := v.(type) {
	case map[string]any:
		for k, item := range vv {
			walkJSONStrings(item, k, fn)
		}
	case []any:
		for _, item := range vv {
			walkJSONStrings(item, key, fn)
		}
	case string:
		fn(key, vv)
	}
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
