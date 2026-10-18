// Command agentmanifest is the offline exposure inventory for coding-agent
// CLIs, third-party skills, and MCP servers. The default scan path performs
// zero network calls: reachability is derived from a static key-shape →
// vendor → endpoint mapping, and credentials are recorded as SHA-256
// fingerprints only.
//
// Release builds may stamp the version with:
//
//	go build -ldflags "-X main.version=$(cat VERSION)" ./cmd/agentmanifest
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SuperMarioYL/agentmanifest/internal/collectors"
	"github.com/SuperMarioYL/agentmanifest/internal/manifest"
)

// version is the release version; the repo-root VERSION file is the single
// release source and is injected here via -ldflags at build time.
var version = "0.1.0"

const (
	exitOK   = 0
	exitErr  = 1
	exitStub = 2 // command exists but is a roadmap stub in this release
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return usage(stdout)
	}
	switch args[0] {
	case "scan":
		return cmdScan(args[1:], stdout, stderr)
	case "report":
		fmt.Fprintln(stderr, "report: not implemented in this release (roadmap m2).")
		fmt.Fprintln(stderr, "        available today: agentmanifest scan")
		return exitStub
	case "scope":
		fmt.Fprintln(stderr, "scope: not implemented in this release (roadmap m2).")
		fmt.Fprintln(stderr, "       available today: agentmanifest scan")
		return exitStub
	case "diff":
		fmt.Fprintln(stderr, "diff: not implemented in this release (roadmap m3).")
		fmt.Fprintln(stderr, "      available today: agentmanifest scan")
		return exitStub
	case "version":
		fmt.Fprintf(stdout, "agentmanifest %s (manifest schema %s)\n", version, manifest.SchemaVersion)
		return exitOK
	case "help", "-h", "--help":
		return usage(stdout)
	default:
		fmt.Fprintf(stderr, "agentmanifest: unknown command %q\n\n", args[0])
		return usage(stderr)
	}
}

func usage(w io.Writer) int {
	fmt.Fprintf(w, `agentmanifest %s — offline exposure inventory for coding-agent toolchains

Usage:

  agentmanifest scan  [flags]      scan known agent config paths, write exposure-manifest.yaml/json
  agentmanifest report <file>      render the archivable admission report (roadmap m2)
  agentmanifest scope  --provider  scope keys to rotate for one vendor (roadmap m2)
  agentmanifest diff   old new     diff two scans for silent drift (roadmap m3)
  agentmanifest version            print version and manifest schema version

Scan flags:

  -out string     output directory (default ".")
  -format string  yaml | json | both (default "both")
  -home string    home directory holding global agent configs (default: $HOME)
  -project string project directory with opencode.json / .mcp.json / skills (default: cwd)

The scanner is read-only and makes zero network calls; credentials are
recorded as sha256 fingerprints, never plaintext.
`, version)
	return exitOK
}

func cmdScan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outDir := fs.String("out", ".", "output directory")
	format := fs.String("format", "both", "yaml | json | both")
	home := fs.String("home", "", "home directory holding global agent configs")
	project := fs.String("project", "", "project directory with opencode.json / .mcp.json / skills")
	if err := fs.Parse(args); err != nil {
		return exitErr
	}
	switch strings.ToLower(*format) {
	case "yaml", "json", "both":
		*format = strings.ToLower(*format)
	default:
		fmt.Fprintf(stderr, "scan: unknown -format %q (want yaml, json or both)\n", *format)
		return exitErr
	}

	opts := collectors.Options{Home: *home, Project: *project}
	m, err := collectors.Run(opts)
	if err != nil {
		fmt.Fprintf(stderr, "scan: %v\n", err)
		return exitErr
	}
	if err := m.Validate(); err != nil {
		fmt.Fprintf(stderr, "scan: produced an invalid manifest: %v\n", err)
		return exitErr
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "scan: %v\n", err)
		return exitErr
	}
	written := 0
	if *format == "yaml" || *format == "both" {
		if err := writeFile(filepath.Join(*outDir, "exposure-manifest.yaml"), m.MarshalYAMLBytes); err != nil {
			fmt.Fprintf(stderr, "scan: %v\n", err)
			return exitErr
		}
		written++
	}
	if *format == "json" || *format == "both" {
		if err := writeFile(filepath.Join(*outDir, "exposure-manifest.json"), m.MarshalJSONBytes); err != nil {
			fmt.Fprintf(stderr, "scan: %v\n", err)
			return exitErr
		}
		written++
	}

	stats := summarize(m)
	fmt.Fprintf(stdout, "scanned host %s: %s\n", m.HostFingerprint, stats)
	for _, name := range []string{"exposure-manifest.yaml", "exposure-manifest.json"} {
		if *format == "json" && name == "exposure-manifest.yaml" {
			continue
		}
		if *format == "yaml" && name == "exposure-manifest.json" {
			continue
		}
		fmt.Fprintf(stdout, "wrote %s\n", filepath.Join(*outDir, name))
	}
	return exitOK
}

func writeFile(path string, marshal func() ([]byte, error)) error {
	data, err := marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func summarize(m manifest.Manifest) string {
	var creds, reach, declared, observed int
	for _, t := range m.Tools {
		creds += len(t.Credentials)
		reach += len(t.Reachability)
		declared += len(t.DeclaredEgress)
		observed += len(t.ObservedEgress)
	}
	return fmt.Sprintf(
		"%d tools, %d credentials (sha256 fingerprints only), %d reachable providers, %d declared egress domains, %d observed",
		len(m.Tools), creds, reach, declared, observed)
}
