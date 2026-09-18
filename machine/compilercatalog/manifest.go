// Manifest validation checks the build identity and every supplied artifact.
// It never treats a path or a claimed digest as a substitute for bytes.
package compilercatalog

import "fmt"

const supportedTarget = "riscv64gc-unknown-linux-gnu"

func validateManifest(build BuildManifest) error {
	if build.CompilationID == "" {
		return fmt.Errorf("empty build compilation identity")
	}
	if build.Target != supportedTarget {
		return fmt.Errorf("unsupported target: %s", build.Target)
	}
	if build.Inventory.Path == "" {
		return fmt.Errorf("empty inventory artifact path")
	}
	if build.Object.Path == "" {
		return fmt.Errorf("empty object artifact path")
	}
	if build.ELF.Path == "" {
		return fmt.Errorf("empty ELF artifact path")
	}
	return nil
}

func validateInventoryArtifact(content []byte, expected ArtifactIdentity) error {
	if err := sizeMatches(content, expected.Size, "inventory"); err != nil {
		return err
	}
	return hashMatches(content, expected.SHA256, "inventory")
}
