// CBMC JSON contains status records and one loop collection for a GOTO model.
// Missing, duplicate, or malformed collections are explicit model errors.

use super::{Error, GotoLoop};
use serde::Deserialize;

#[derive(Deserialize)]
struct Entry {
    loops: Option<Vec<GotoLoop>>,
}

pub fn decode_cbmc_loop_inventory(origin: &str, input: &[u8]) -> Result<Vec<GotoLoop>, Error> {
    let entries: Vec<Entry> = serde_json::from_slice(input).map_err(|error| Error::Json {
        origin: origin.to_string(),
        reason: error.to_string(),
    })?;
    let mut inventories = entries.into_iter().filter_map(|entry| entry.loops);
    let loops = inventories.next().ok_or_else(|| Error::Model {
        reason: format!("{origin}: CBMC returned no loop inventory"),
    })?;
    if inventories.next().is_some() {
        return Err(Error::Model {
            reason: format!("{origin}: CBMC returned more than one loop inventory"),
        });
    }
    Ok(loops)
}
