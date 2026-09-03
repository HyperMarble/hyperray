// BOUND: for every changed function, say what could make a proof run
// forever (a loop, a growable input) and where its finite limit comes
// from. Phase A sorts; later phases attach the limit.

mod adt;
mod binop;
mod block;
mod constant;
mod decl;
mod edge;
mod kind;
mod limit;
mod pile;
mod row;
mod size;
mod sort;
mod stmt;
mod ty;
mod ullbc;

pub use binop::BinOp;
pub use kind::{Bound, Row};
pub use limit::Limit;
pub use pile::{pile, Pile};
pub use row::rows;
pub use size::Size;
pub use sort::{sort_all, Sorted};
pub use ullbc::{read, Output};
