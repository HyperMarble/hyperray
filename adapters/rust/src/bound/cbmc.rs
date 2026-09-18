// CBMC reads a final linked GOTO model and returns its complete loop inventory.
// This stage requests model data only; it does not start a proof.

use super::{decode_cbmc_loop_inventory, tool, Error, GotoLoop};
use std::path::Path;
use std::process::Command;

pub fn inventory(program: &Path, goto_file: &Path) -> Result<Vec<GotoLoop>, Error> {
    let mut command = Command::new(program);
    command.arg(goto_file).args(["--show-loops", "--json-ui"]);
    let output = tool::run(&mut command, "CBMC")?;
    decode_cbmc_loop_inventory(&goto_file.display().to_string(), &output.stdout)
}

pub fn version(program: &Path) -> Result<String, Error> {
    tool::version(program, &["--version"], "CBMC")
}
