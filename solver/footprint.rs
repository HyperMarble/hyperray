// Purpose: reads an instruction's facts out of the model's own footprint.
// Never:   decides a fact the footprint did not state.
// In:      a Footprint from the architecture model
// Out:     Facts, with the model's register names
// Fails:   the model does not describe the instruction
use crate::facts::{Facts, FactsError};
use isla_axiomatic::footprint_analysis::Footprint;

/// Reads one instruction's facts out of a footprint the model produced.
///
/// The footprint is the model's own record of the instruction, so this
/// function moves its fields rather than deciding anything.
pub fn from_footprint(footprint: &Footprint, name_of: impl Fn(&str) -> String) -> Facts {
    Facts {
        register_reads: named(&footprint.register_reads, &name_of),
        register_writes: named(&footprint.register_writes, &name_of),
        is_load: footprint.is_load,
        is_store: footprint.is_store,
        is_branch: footprint.is_branch,
    }
}

/// The model holds interned names, so each is turned back into its text.
fn named<T>(fields: &std::collections::HashSet<T>, name_of: &impl Fn(&str) -> String) -> Vec<String>
where
    T: std::fmt::Debug,
{
    let mut names: Vec<String> = fields.iter().map(|field| name_of(&format!("{field:?}"))).collect();
    names.sort();
    names
}

/// Reports that the model was not asked about this instruction.
pub fn unknown(opcode: u32) -> FactsError {
    FactsError::Unknown { opcode }
}
