// Package drift defines the comparison of two ExposureManifests taken at
// different times: silent skill updates, newly added credentials, and egress
// changes between scans.
//
// Stub status (roadmap m3): the DriftReport shapes below fix the output of
// `agentmanifest diff old.yaml new.yaml`; the diff logic, the 等保 2.0
// supply-chain review fields, and local ed25519 signing land with m3. The
// v0.1.0 binary prints a roadmap notice for this command.
package drift

// Change is one difference between two scans.
type Change struct {
	ToolName string
	Kind     string // credential_added | credential_removed | egress_added | egress_removed
	Detail   string
}

// DriftReport is the m3 archive artifact: the diff of two manifests, plus the
// supply-chain review fields, ready for local ed25519 signing.
type DriftReport struct {
	OldGeneratedAt string
	NewGeneratedAt string
	Changes        []Change
}
