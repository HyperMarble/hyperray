// Join reconciles actual inventory, object bytes, and one linked ELF.
// It reports observations and obligations, never ValidatedCoverage.
package compilercatalog

import (
	"bytes"
	"debug/elf"
	"fmt"

	"github.com/HyperMarble/hyperray/machine"
)

func Join(artifact Artifact, build BuildManifest, objects []ObjectArtifact, elfContent []byte, maximumLoadedBytes uint64) (Report, error) {
	measured, err := Read(artifact.Content)
	if err != nil {
		return Report{}, fmt.Errorf("re-read inventory content: %w", err)
	}
	artifact = measured
	if build.Version != 1 {
		return Report{}, fmt.Errorf("unsupported build manifest version: %d", build.Version)
	}
	if err := validateManifest(build); err != nil {
		return Report{}, err
	}
	if artifact.Inventory.CompilationID != build.CompilationID {
		return Report{}, fmt.Errorf("cross-build inventory: %s != %s", artifact.Inventory.CompilationID, build.CompilationID)
	}
	if err := validateInventoryArtifact(artifact.Content, build.Inventory); err != nil {
		return Report{}, err
	}
	if err := validateObjectInputs(objects, build); err != nil {
		return Report{}, err
	}
	object, err := selectObject(objects, build.Object)
	if err != nil {
		return Report{}, err
	}
	if err := validateArtifactBytes(elfContent, build.ELF, "ELF"); err != nil {
		return Report{}, err
	}
	image, err := machine.Load(elfContent, maximumLoadedBytes)
	if err != nil {
		return Report{}, fmt.Errorf("load linked ELF: %w", err)
	}
	objectNames, err := symbols(object.Content)
	if err != nil {
		return Report{}, fmt.Errorf("read object symbols: %w", err)
	}
	imageNames, err := symbols(elfContent)
	if err != nil {
		return Report{}, fmt.Errorf("read image symbols: %w", err)
	}
	report := Report{CompilationID: build.CompilationID, Image: image, ObjectSymbols: objectNames, ImageSymbols: imageNames, UnresolvedObligations: append([]string(nil), build.UnresolvedObligations...)}
	appendInventory(&report, artifact.Inventory)
	if hasOpaqueBodyPayload(artifact.Inventory) {
		report.UnresolvedObligations = append(report.UnresolvedObligations, "structural raw-body coverage remains unresolved")
	}
	if hasOpaqueRootFacts(artifact.Inventory) {
		report.UnresolvedObligations = append(report.UnresolvedObligations, "root ABI facts remain unresolved")
	}
	report.Associations = associateSymbols(objectNames, imageNames)
	report.UnresolvedObligations = append(report.UnresolvedObligations, "symbol and range associations are not operation-to-instruction proofs", "linker-origin evidence remains unresolved", "source and runtime input closure remains unresolved", "selected sysroot identity remains unresolved", "pre-optimization operation preservation remains unresolved")
	return report, nil
}

func selectObject(objects []ObjectArtifact, expected ArtifactIdentity) (ObjectArtifact, error) {
	var selected ObjectArtifact
	found := false
	seen := make(map[string]struct{}, len(objects))
	for _, object := range objects {
		if _, exists := seen[object.Path]; exists {
			return ObjectArtifact{}, fmt.Errorf("duplicate object artifact: %s", object.Path)
		}
		seen[object.Path] = struct{}{}
		if object.Path != expected.Path {
			continue
		}
		if found {
			return ObjectArtifact{}, fmt.Errorf("duplicate object artifact: %s", object.Path)
		}
		selected = object
		found = true
	}
	if !found {
		return ObjectArtifact{}, fmt.Errorf("missing object artifact: %s", expected.Path)
	}
	return selected, nil
}

func appendInventory(report *Report, inventory Inventory) {
	for _, instance := range inventory.Instances {
		report.Roots = append(report.Roots, RootObligation{InstanceID: instance.ID, Reasons: append([]string(nil), instance.RootObligations...)})
		if instance.Body == nil {
			continue
		}
		for _, position := range instance.Body.Positions {
			report.Operations = append(report.Operations, Operation{ID: position.OperationID, InstanceID: instance.ID, Block: position.Block, Statement: position.Statement, Terminator: position.Terminator || position.Statement == nil, Payload: append([]byte(nil), position.Payload...), SourceSpan: append([]byte(nil), position.SourceSpan...)})
		}
	}
}

func associateSymbols(objects []string, images []string) []SymbolAssociation {
	imageSet := make(map[string]struct{}, len(images))
	for _, image := range images {
		imageSet[image] = struct{}{}
	}
	associations := make([]SymbolAssociation, 0)
	for _, object := range objects {
		if _, exists := imageSet[object]; !exists {
			continue
		}
		associations = append(associations, SymbolAssociation{Symbol: object, Evidence: "symbol name only; no operation mapping"})
	}
	return associations
}

func openELF(content []byte) (*elf.File, error) {
	return elf.NewFile(bytes.NewReader(content))
}
