// A rustc driver that writes MIR schema 3. A write or translation error must
// make the compiler process fail.
#![feature(rustc_private)]

extern crate rustc_driver;
extern crate rustc_interface;
extern crate rustc_middle;
extern crate rustc_public;

mod block;
mod input;
mod input_type;
mod instance;
mod instance_kind;
mod inventory;
mod item;
mod operand;
mod operation;
mod out;
mod place;
mod read;
mod rvalue;
mod scalar;
mod terminator;
mod value;

use rustc_middle::ty::TyCtxt;
use rustc_public::CompilerError;
use std::ops::ControlFlow;
use std::sync::atomic::{AtomicBool, Ordering};

static WRITE_FAILED: AtomicBool = AtomicBool::new(false);

fn dump(tcx: TyCtxt<'_>) -> ControlFlow<()> {
    let result = if std::env::var_os("HYPERRAY_INVENTORY_DIR").is_some() {
        inventory::write(tcx)
    } else if std::env::var_os("CARGO_PRIMARY_PACKAGE").is_none() {
        return ControlFlow::Continue(());
    } else {
        item::dump(tcx).and_then(|dump| out::write(&dump))
    };
    match result {
        Ok(path) => eprintln!("mir-dump: wrote {}", path.display()),
        Err(error) => {
            eprintln!("mir-dump: {error}");
            WRITE_FAILED.store(true, Ordering::Relaxed);
            return ControlFlow::Break(());
        }
    }
    ControlFlow::Continue(())
}

fn main() {
    let arguments = compiler_arguments();
    let result = rustc_public::run_with_tcx!(&arguments, dump);
    let compiler_failed = matches!(result, Err(CompilerError::Failed));
    if compiler_failed || WRITE_FAILED.load(Ordering::Relaxed) {
        std::process::exit(1);
    }
}

fn compiler_arguments() -> Vec<String> {
    let mut arguments: Vec<String> = std::env::args().collect();
    if std::env::var_os("HYPERRAY_RUSTC_WRAPPER").is_some() && arguments.len() > 1 {
        arguments.remove(1);
    }
    arguments
}
