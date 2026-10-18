// Package scope defines the rotation-scoping layer: given a provider in the
// middle of an incident, it answers "which of MY tools' key fingerprints must
// be rotated" instead of "rotate all keys".
//
// Stub status (roadmap m2): the data shapes below fix the output contract of
// `agentmanifest scope --provider <p>`; the selection logic and CLI rendering
// land with m2. The v0.1.0 binary prints a roadmap notice for this command.
package scope

// RotationItem is one key fingerprint that an incident on `provider` puts in
// the rotation set, together with the tool that holds it.
type RotationItem struct {
	ToolName       string
	InstallPath    string
	Provider       string
	KeyFingerprint string
	KeyHint        string
	SourceFile     string
}
