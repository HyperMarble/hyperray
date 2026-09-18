// The Stage 3 builder joins rustc source evidence to Kani and CBMC model data.
// It returns no result until every configured compiler and tool step succeeds.

use super::model::{HarnessMode, Model, Scope, ToolVersions};
use super::{analyze_dump, cbmc, contract, fresh, harness, kani, metadata, rows, selection, Error};
use crate::extract::Joined;
use crate::mir::Dump;

pub fn build_model(manifest: &[Joined], dumps: &[Dump], scope: &Scope<'_>) -> Result<Model, Error> {
    let mut items = Vec::new();
    for dump in dumps {
        items.extend(analyze_dump(dump)?);
    }
    let rows = rows(manifest, &items)?;
    let tools = ToolVersions {
        kani: kani::version(scope.cargo)?,
        cbmc: cbmc::version(scope.cbmc)?,
    };
    let artifact_dir = fresh::directory(scope.output_dir)?;
    kani::generate(scope, &artifact_dir)?;
    let metadata = metadata::read_all(&artifact_dir)?;
    let automatic_selections = selection::all(&metadata);
    let contracts = contract::all(&metadata);
    if scope.harness_mode == HarnessMode::Automatic && automatic_selections.is_empty() {
        return Err(Error::Model {
            reason: "Kani autoharness produced no selection report".to_string(),
        });
    }
    let harnesses = harness::models(&metadata, scope.cbmc)?;
    Ok(Model {
        rows,
        harnesses,
        contracts,
        automatic_selections,
        tools,
        artifact_dir,
        harness_mode: scope.harness_mode,
        kani_arguments: scope.kani_arguments.to_vec(),
    })
}
