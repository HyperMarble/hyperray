// Public catalog types carry compiler positions, build identities, and unresolved obligations.
// They never claim semantic coverage or convert a symbol match into proof.
package compilercatalog

import (
	"encoding/json"

	"github.com/HyperMarble/hyperray/machine"
)

type Inventory struct {
	Version       uint32     `json:"version"`
	CompilationID string     `json:"compilation_id"`
	Instances     []Instance `json:"instances"`
}

type Instance struct {
	ID               string          `json:"id"`
	Symbol           string          `json:"symbol"`
	Name             string          `json:"name"`
	Kind             string          `json:"kind"`
	GenericArguments []string        `json:"generic_arguments"`
	ABI              json.RawMessage `json:"abi"`
	ABIError         string          `json:"abi_error"`
	BodyStatus       string          `json:"body_status"`
	Body             *Body           `json:"body"`
	RootObligations  []string        `json:"root_obligations"`
	RootFacts        json.RawMessage `json:"root_facts"`
}

type Body struct {
	Phase      string          `json:"phase"`
	BodyDigest string          `json:"body_digest"`
	Positions  []Position      `json:"positions"`
	Payload    json.RawMessage `json:"payload"`
}

type Position struct {
	Block             int             `json:"block"`
	Statement         *int            `json:"statement"`
	Terminator        bool            `json:"terminator"`
	OperationID       string          `json:"operation_id"`
	Payload           json.RawMessage `json:"payload"`
	SourceSpan        json.RawMessage `json:"source_span"`
	terminatorPresent bool
	statementPresent  bool
}

type Artifact struct {
	Inventory Inventory
	Content   []byte
	SHA256    string
}

type ArtifactIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   uint64 `json:"size"`
}

type ToolIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type BuildManifest struct {
	Version               uint32             `json:"version"`
	CompilationID         string             `json:"compilation_id"`
	Target                string             `json:"target"`
	Driver                ToolIdentity       `json:"driver"`
	Linker                ToolIdentity       `json:"linker"`
	Source                ArtifactIdentity   `json:"source"`
	BoundaryArtifact      *ArtifactIdentity  `json:"boundary_artifact"`
	ExternArtifacts       []ArtifactIdentity `json:"extern_artifacts"`
	Object                ArtifactIdentity   `json:"object"`
	DepInfo               ArtifactIdentity   `json:"dep_info"`
	Inventory             ArtifactIdentity   `json:"inventory"`
	ELF                   ArtifactIdentity   `json:"elf"`
	CompilerArguments     []string           `json:"compiler_arguments"`
	LinkerArguments       []string           `json:"linker_arguments"`
	UnresolvedObligations []string           `json:"unresolved_obligations"`
}

type ObjectArtifact struct {
	Path    string
	Content []byte
}

type Report struct {
	CompilationID         string
	Operations            []Operation
	Roots                 []RootObligation
	ObjectSymbols         []string
	ImageSymbols          []string
	Associations          []SymbolAssociation
	Image                 machine.Image
	UnresolvedObligations []string
}

type Operation struct {
	ID         string
	InstanceID string
	Block      int
	Statement  *int
	Terminator bool
	Payload    json.RawMessage
	SourceSpan json.RawMessage
}

type RootObligation struct {
	InstanceID string
	Reasons    []string
}

type SymbolAssociation struct {
	Symbol        string
	ObjectAddress uint64
	ImageAddress  uint64
	Evidence      string
}
