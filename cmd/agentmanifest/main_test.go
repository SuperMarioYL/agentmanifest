package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fixtureRoot = func() string {
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures"))
	if err != nil {
		panic(err)
	}
	return root
}()

func TestScanCommandWritesBothFormats(t *testing.T) {
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"scan",
		"-home", filepath.Join(fixtureRoot, "home"),
		"-project", fixtureRoot,
		"-out", outDir,
		"-format", "both",
	}, &stdout, &stderr)

	require.Equal(t, exitOK, code, "stderr: %s", stderr.String())
	out := stdout.String()
	assert.Contains(t, out, "tools,")
	assert.Contains(t, out, "sha256 fingerprints only")

	yamlFile := filepath.Join(outDir, "exposure-manifest.yaml")
	jsonFile := filepath.Join(outDir, "exposure-manifest.json")

	yamlData, err := os.ReadFile(yamlFile)
	require.NoError(t, err, "expected %s to be written", yamlFile)
	assert.Contains(t, string(yamlData), "schema_version: v0")

	jsonData, err := os.ReadFile(jsonFile)
	require.NoError(t, err, "expected %s to be written", jsonFile)
	assert.Contains(t, string(jsonData), `"schema_version": "v0"`)
}

func TestScanFormatFlags(t *testing.T) {
	outDir := t.TempDir()
	for _, format := range []string{"yaml", "json"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{
			"scan", "-home", filepath.Join(fixtureRoot, "home"),
			"-project", fixtureRoot, "-out", outDir, "-format", format,
		}, &stdout, &stderr)
		require.Equal(t, exitOK, code, "stderr: %s", stderr.String())
	}
	entries, err := os.ReadDir(outDir)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "yaml + json runs produce exactly two files")

	var stdout, stderr bytes.Buffer
	code := run([]string{"scan", "-format", "xml"}, &stdout, &stderr)
	assert.Equal(t, exitErr, code)
	assert.Contains(t, stderr.String(), "unknown -format")
}

func TestStubCommandsExitWithRoadmapNotice(t *testing.T) {
	for _, cmd := range []string{"report", "scope", "diff"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{cmd}, &stdout, &stderr)
		assert.Equal(t, exitStub, code, "%s should exit as a stub", cmd)
		assert.Contains(t, stderr.String(), "roadmap")
	}
}

func TestVersionCommand(t *testing.T) {
	var stdout, _ bytes.Buffer
	code := run([]string{"version"}, &stdout, os.Stderr)
	require.Equal(t, exitOK, code)
	assert.Contains(t, stdout.String(), "agentmanifest")
	assert.Contains(t, stdout.String(), "manifest schema v0")
}

func TestUsageAndUnknownCommand(t *testing.T) {
	var stdout, _ bytes.Buffer
	assert.Equal(t, exitOK, run([]string{"help"}, &stdout, os.Stderr))
	assert.Contains(t, stdout.String(), "agentmanifest scan")

	var stderr bytes.Buffer
	assert.Equal(t, exitOK, run(nil, os.Stdout, &stderr))
	assert.Equal(t, exitOK, run([]string{"--help"}, os.Stdout, &stdout))
	assert.Equal(t, exitOK, run([]string{"bogus-command"}, os.Stdout, &stderr))
	assert.Contains(t, stderr.String(), "unknown command")
}
