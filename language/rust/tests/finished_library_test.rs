// Purpose: a library is also handed over in its finished form, the one Cargo
//          links, and every function in its `.rlib` is in that finished file.
// Never:   decides by hand which functions a library holds: `nm` reads both.
mod library_files;
mod sample_project;

use library_files::{
    file_of_kind,
    functions,
    record_of, //
};

const LIBRARY: &str =
    "pub fn total(each: u64, count: u64) -> u64 {\n    helper(each) * count\n}\n\n\
fn helper(x: u64) -> u64 {\n    x + 1\n}\n\npub struct Shelf {\n    pub left: u64,\n}\n\n\
impl Shelf {\n    pub fn low(&self) -> bool {\n        self.left < 5\n    }\n}\n";

#[test]
fn every_function_in_a_library_is_in_its_finished_form() -> Result<(), String> {
    let record = record_of("finished_whole", LIBRARY)?;
    let library = file_of_kind(&record, "lib").ok_or("the library file is not in the record")?;
    let finished = file_of_kind(&record, "finished library");
    let finished = finished.ok_or("no finished library in the record")?;
    let wanted = functions(&library, "finished_whole")?;
    let found = functions(&finished, "finished_whole")?;
    assert!(wanted.len() >= 3, "nm found {wanted:?} in {library}");
    let missing: Vec<&String> = wanted.difference(&found).collect();
    assert!(
        missing.is_empty(),
        "missing from the finished library: {missing:?}"
    );
    Ok(())
}
