// Harness conversion validates crate ownership before CBMC reads each model.
// The result contains every harness and every loop in deterministic order.

use super::metadata::{CrateMetadata, HarnessMetadata};
use super::{artifact, cbmc, Error, HarnessClass, HarnessModel};
use std::path::Path;

pub fn models(metadata: &[CrateMetadata], cbmc: &Path) -> Result<Vec<HarnessModel>, Error> {
    let mut models = Vec::new();
    for package in metadata {
        for harness in &package.proof_harnesses {
            models.push(model(package, harness, HarnessClass::Proof, cbmc)?);
        }
        for harness in &package.test_harnesses {
            models.push(model(package, harness, HarnessClass::Test, cbmc)?);
        }
    }
    if models.is_empty() {
        return Err(Error::Model {
            reason: "Kani produced no proof harness models".to_string(),
        });
    }
    models.sort_by(|left, right| {
        (&left.crate_name, &left.harness).cmp(&(&right.crate_name, &right.harness))
    });
    Ok(models)
}

fn model(
    package: &CrateMetadata,
    harness: &HarnessMetadata,
    class: HarnessClass,
    cbmc_program: &Path,
) -> Result<HarnessModel, Error> {
    if package.crate_name != harness.crate_name {
        return Err(Error::Model {
            reason: format!(
                "{}: harness {} belongs to crate {}, not {}",
                package.source.display(),
                harness.pretty_name,
                harness.crate_name,
                package.crate_name
            ),
        });
    }
    let source = harness.goto_file.as_ref().ok_or_else(|| Error::Model {
        reason: format!(
            "{}: harness {} has no GOTO model",
            package.crate_name, harness.pretty_name
        ),
    })?;
    let goto_file = artifact::linked(source)?;
    let loops = cbmc::inventory(cbmc_program, &goto_file)?;
    Ok(HarnessModel {
        crate_name: harness.crate_name.clone(),
        harness: harness.pretty_name.clone(),
        class,
        original_file: harness.original_file.clone(),
        original_start_line: harness.original_start_line,
        original_end_line: harness.original_end_line,
        automatically_generated: harness.is_automatically_generated,
        attributes: harness.attributes.clone(),
        contract: harness.contract.clone(),
        has_loop_contracts: harness.has_loop_contracts,
        goto_file,
        loops,
    })
}
