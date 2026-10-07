// Purpose: a build where the compiler itself prints text (here a proc macro
//          printing while it runs) is still recorded; that text is not one
//          of Cargo's messages and is skipped.
// Never:   blocks a build because the compiler printed something.
mod sample_project;

use language_rust::build_record;
use language_rust::record::Outcome;
use sample_project::{create, Sample};
use std::io::Write;

const PRINTER: &str = "use proc_macro::TokenStream;\n#[proc_macro]\n\
pub fn noisy(_input: TokenStream) -> TokenStream {\n\
    println!(\"printed by the compiler while building\");\n    TokenStream::new()\n}\n";

#[test]
fn text_printed_by_the_compiler_does_not_block_the_record() -> Result<(), String> {
    let root = create(&Sample {
        name: "printer_user",
        file: "main.rs",
        source: "printer::noisy!();\nfn main() {}\n",
        locked: false,
    })?;
    let printer = root.join("printer");
    std::fs::create_dir_all(printer.join("src")).map_err(|e| e.to_string())?;
    let manifest = "[package]\nname = \"printer\"\nversion = \"0.1.0\"\nedition = \"2021\"\n[lib]\nproc-macro = true\n";
    std::fs::write(printer.join("Cargo.toml"), manifest).map_err(|e| e.to_string())?;
    std::fs::write(printer.join("src/lib.rs"), PRINTER).map_err(|e| e.to_string())?;
    let mut own = std::fs::OpenOptions::new()
        .append(true)
        .open(root.join("Cargo.toml"))
        .map_err(|e| e.to_string())?;
    own.write_all(b"\n[dependencies]\nprinter = { path = \"printer\" }\n")
        .map_err(|e| e.to_string())?;
    let locked = std::process::Command::new("cargo")
        .args(["generate-lockfile", "--offline"])
        .current_dir(&root)
        .status();
    assert!(locked.map_err(|e| e.to_string())?.success());
    let outcome = build_record(&root);
    let Outcome::Built(record) = outcome else {
        return Err(format!(
            "the compiler's printout blocked the record: {outcome:?}"
        ));
    };
    assert!(record.artifacts.iter().any(|a| a.kind == "bin"));
    Ok(())
}
