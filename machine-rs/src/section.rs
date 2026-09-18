// A relocatable object states its contents in sections, not segments.
use crate::permission;
use crate::region::Region;
use object::{Object, ObjectSection};

/// Every section that occupies memory, for a file that has no segments.
///
/// A relocatable object states its contents in sections only. Reading the
/// segments alone would report such a file as holding nothing.
pub fn regions(file: &object::File<'_>) -> Result<Vec<Region>, String> {
    let mut found = Vec::new();
    for section in file.sections() {
        if !occupies_memory(&section) {
            continue;
        }
        let supplied = section.data().map_err(|error| error.to_string())?;
        let permission = permission::of_section(section.kind());
        let Some(region) =
            crate::region::occupied(section.address(), section.size(), supplied, permission)?
        else {
            continue;
        };
        found.push(region);
    }
    Ok(found)
}

fn occupies_memory(section: &object::Section<'_, '_>) -> bool {
    use object::SectionKind;
    matches!(
        section.kind(),
        SectionKind::Text
            | SectionKind::Data
            | SectionKind::ReadOnlyData
            | SectionKind::ReadOnlyString
            | SectionKind::UninitializedData
    )
}
