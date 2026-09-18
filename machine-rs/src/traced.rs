// Semantics already obtained for an instruction encoding. The same encoding
// has the same semantics, so tracing it twice is wasted work.
use crate::engine::Engine;
use crate::footprint::{self, Footprint};
use std::collections::HashMap;

/// Footprints kept by encoding for the life of one proof.
#[derive(Default)]
pub struct Traced {
    known: HashMap<[u8; 4], Footprint>,
    hits: usize,
    misses: usize,
}

impl Traced {
    /// An empty store.
    pub fn new() -> Traced {
        Traced::default()
    }

    /// The footprint of `opcode`, traced only if it has not been seen.
    ///
    /// A failure is returned rather than stored, so a later attempt can
    /// report it again.
    pub fn footprint(&mut self, engine: &Engine, opcode: [u8; 4]) -> Result<&Footprint, String> {
        if !self.known.contains_key(&opcode) {
            self.misses += 1;
            let traced = footprint::trace(engine, &opcode)?;
            self.known.insert(opcode, traced);
        } else {
            self.hits += 1;
        }
        self.known
            .get(&opcode)
            .ok_or_else(|| format!("footprint for {opcode:02x?} was not stored"))
    }

    /// How many requests were answered from the store, and how many were traced.
    pub fn counts(&self) -> (usize, usize) {
        (self.hits, self.misses)
    }
}
