// Package render defines the archivable admission report rendered from an
// ExposureManifest — the 等保-style document a reviewer can file as-is.
//
// Stub status (roadmap m2): the three report tables below fix the layout of
// `agentmanifest report <manifest.yaml>`; the markdown/HTML rendering lands
// with m2. The v0.1.0 binary prints a roadmap notice for this command.
package render

// ReportRow is one row of a report table.
type ReportRow struct {
	Tool    string
	Details []string
}

// Report carries the three tables of the admission report:
//
//  1. CredentialsTable — which keys (fingerprints only) each tool holds.
//  2. EgressTable     — which domains each tool is configured to contact.
//  3. ContentTable    — which user content classes may leave, to where.
type Report struct {
	GeneratedAt      string
	HostFingerprint  string
	CredentialsTable []ReportRow
	EgressTable      []ReportRow
	ContentTable     []ReportRow
}
