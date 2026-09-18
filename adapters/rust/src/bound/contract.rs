// Contract records keep each Kani function-to-harness relationship visible.
// They do not claim that a contract is valid before Stage 4 proves it.

use super::metadata::CrateMetadata;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ContractedFunction {
    pub crate_name: String,
    pub function: String,
    pub file: String,
    pub harnesses: Vec<String>,
}

pub(crate) fn all(metadata: &[CrateMetadata]) -> Vec<ContractedFunction> {
    let mut contracts: Vec<ContractedFunction> = metadata
        .iter()
        .flat_map(|package| {
            package
                .contracted_functions
                .iter()
                .map(|contract| ContractedFunction {
                    crate_name: package.crate_name.clone(),
                    function: contract.function.clone(),
                    file: contract.file.clone(),
                    harnesses: contract.harnesses.clone(),
                })
        })
        .collect();
    contracts.sort_by(|left, right| {
        (&left.crate_name, &left.function).cmp(&(&right.crate_name, &right.function))
    });
    contracts
}
