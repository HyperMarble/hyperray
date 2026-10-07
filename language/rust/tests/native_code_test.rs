// Purpose: C code that a build script compiles into the program is recorded,
//          with the libraries Cargo reports it linked.
// Never:   reports a build as pure Rust when native code was linked in.
mod sample_project;

use language_rust::build_record;
use language_rust::record::Outcome;
use sample_project::{create, Sample};

const BUILD_SCRIPT: &str = "use std::process::Command;\n\
fn main() -> Result<(), Box<dyn std::error::Error>> {\n\
    let out = std::env::var(\"OUT_DIR\")?;\n\
    Command::new(\"cc\").args([\"-c\", \"twice.c\", \"-o\"]).arg(format!(\"{out}/twice.o\")).status()?;\n\
    Command::new(\"ar\").arg(\"rcs\").arg(format!(\"{out}/libtwice.a\")).arg(format!(\"{out}/twice.o\")).status()?;\n\
    println!(\"cargo:rustc-link-search=native={out}\");\n\
    println!(\"cargo:rustc-link-lib=static=twice\");\n\
    Ok(())\n}\n";

const MAIN: &str = "extern \"C\" { fn c_twice(x: i32) -> i32; }\n\
fn main() { println!(\"{}\", unsafe { c_twice(std::hint::black_box(21)) }); }\n";

#[test]
fn c_code_built_into_the_program_is_recorded() -> Result<(), String> {
    let root = create(&Sample {
        name: "native_code",
        file: "main.rs",
        source: MAIN,
        locked: true,
    })?;
    std::fs::write(root.join("build.rs"), BUILD_SCRIPT).map_err(|e| e.to_string())?;
    std::fs::write(
        root.join("twice.c"),
        "int c_twice(int x) { return x * 2; }\n",
    )
    .map_err(|e| e.to_string())?;
    let Outcome::Built(record) = build_record(&root) else {
        return Err("the project with C code was not built".to_string());
    };
    let found = record
        .native_code
        .iter()
        .find(|n| n.package.contains("native_code"))
        .ok_or("no native code recorded")?;
    assert_eq!(found.linked_libs, ["static=twice"]);
    Ok(())
}
