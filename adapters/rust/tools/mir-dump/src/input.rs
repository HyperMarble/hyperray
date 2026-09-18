// Function inputs retain their MIR local and structural compiler type. This
// file never asks rustc for memory layout.

use crate::input_type;
use hyperray_rust::mir::Input;
use rustc_public::mir::Body;

pub fn of(body: &Body) -> Result<Vec<Input>, String> {
    body.arg_locals()
        .iter()
        .enumerate()
        .map(|(index, local)| {
            Ok(Input {
                index,
                local: index + 1,
                ty: input_type::of(local.ty)?,
            })
        })
        .collect()
}
