// ARM execution manifests bind one measured ARM64 capability.
// They must not turn an artifact format label into memory or runtime support.
package isla

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
)

const (
	arm64ExecutionProfile        = "arm64-fetch-only-v1"
	arm64NormalExecutionProfile  = "arm64-normal-s1-fixed-4k-simd-v1"
	arm64ContinuationVersion     = "continuation-boundary-v1"
	arm64PostResetVersion        = "arm64-post-reset-v1"
	arm64TerminalEvidenceVersion = "v1"
	arm64ProgramCounterRegister  = "_PC"
)

type arm64ExecutionManifest struct {
	CapabilityID                string `json:"capability_id"`
	Profile                     string `json:"profile"`
	ExecutionProfile            string `json:"execution_profile"`
	MemoryProfile               string `json:"memory_profile,omitempty"`
	MemoryConfigDigest          string `json:"memory_config_sha256,omitempty"`
	EffectiveStateDigest        string `json:"effective_state_sha256,omitempty"`
	PCRegister                  string `json:"pc_register"`
	ContinuationBoundaryVersion string `json:"continuation_boundary_version"`
	PostResetVersion            string `json:"post_reset_version"`
	TerminalEvidenceVersion     string `json:"terminal_evidence_version"`
	ArchitectureDigest          string `json:"architecture_sha256"`
	ConfigurationDigest         string `json:"configuration_sha256"`
	MemoryModelDigest           string `json:"memory_model_sha256"`
	SolverVersion               string `json:"solver_version"`
	SolverDigest                string `json:"solver_sha256"`
	SemanticVersion             string `json:"semantic_version"`
	SemanticDigest              string `json:"semantic_sha256"`
	FootprintVersion            string `json:"footprint_version"`
	FootprintDigest             string `json:"footprint_sha256"`
}

func readARM64ExecutionManifest(path string) (arm64ExecutionManifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return arm64ExecutionManifest{}, releaseError(err.Error())
	}
	if err := rejectDuplicateObjectKeys(content, "ARM capability manifest"); err != nil {
		return arm64ExecutionManifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var manifest arm64ExecutionManifest
	if err := decoder.Decode(&manifest); err != nil {
		return arm64ExecutionManifest{}, releaseError(err.Error())
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return arm64ExecutionManifest{}, releaseError("manifest has trailing JSON")
	}
	if err := manifest.valid(); err != nil {
		return arm64ExecutionManifest{}, err
	}
	return manifest, nil
}
