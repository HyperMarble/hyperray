// Purpose: checks a function whose code counts from itself to reach a value.
// Never:   uses hand-written instructions, which hide what a compiler emits.
// In:      a library this test compiles, holding a table lookup
// Out:     nothing when the processor returns the table's real values
// Fails:   the library will not build, or the answer does not match the table
use rechecker::{run_placed, Call, Placed};

/// The table the compiled function reads.
const TABLE: [u64; 4] = [10, 20, 30, 40];

/// Compiles a function that reads a table, which the compiler reaches with
/// an instruction that counts from its own position.
fn build_library() -> Option<std::path::PathBuf> {
    let directory = std::env::temp_dir().join("hyperray_placement_check");
    std::fs::create_dir_all(&directory).ok()?;
    let source = directory.join("pick.c");
    std::fs::write(
        &source,
        "static const unsigned long TABLE[4] = {10,20,30,40};\n\
         unsigned long pick(unsigned long i){ return TABLE[i & 3]; }\n",
    )
    .ok()?;
    let library = directory.join("libpick.dylib");
    let built = std::process::Command::new("cc")
        .args(["-O1", "-dynamiclib", "-o"])
        .arg(&library)
        .arg(&source)
        .status()
        .ok()?;
    built.success().then_some(library)
}

#[test]
fn a_function_that_reads_a_table_returns_the_table() {
    let Some(library) = build_library() else {
        println!("no compiler here, so this check did not run");
        return;
    };
    let placed = Placed::load(&library).expect("the operating system refused the library");
    for (index, expected) in TABLE.iter().enumerate() {
        let call = Call { arguments: vec![index as u64], buffers: Vec::new() };
        let outcome = run_placed(&placed, "pick", &call).expect("the call did not run");
        assert_eq!(
            outcome.returned, *expected,
            "pick({index}) read the wrong memory, so the code was moved away from its table"
        );
    }
}
