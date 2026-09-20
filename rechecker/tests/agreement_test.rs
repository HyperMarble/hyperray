// Purpose: checks the rechecker against the value the same code returns
//          when a normal program calls it.
// Never:   trusts a result this file worked out by hand.
// In:      functions compiled here, called two separate ways
// Out:     nothing when both ways return the same value
// Fails:   the two ways disagree, which means the rechecker moved the code
use rechecker::{run_placed, Call, Placed};

/// Functions whose answers depend on where their code sits.
const SOURCE: &str = r#"
static const unsigned long TABLE[8] = {1,2,4,8,16,32,64,128};
static const char TEXT[] = "hyperray";
unsigned long pick(unsigned long i){ return TABLE[i & 7]; }
unsigned long letter(unsigned long i){ return (unsigned char)TEXT[i & 7]; }
unsigned long add_two(unsigned long a, unsigned long b){ return a + b; }
unsigned long branchy(unsigned long a){
    switch (a & 3) { case 0: return 100; case 1: return 200;
                     case 2: return TABLE[5]; default: return TEXT[2]; }
}
"#;

/// A normal program that calls the same functions and prints the answers.
const CALLER: &str = r#"
#include <stdio.h>
unsigned long pick(unsigned long);
unsigned long letter(unsigned long);
unsigned long add_two(unsigned long, unsigned long);
unsigned long branchy(unsigned long);
int main(void){
    for (unsigned long i = 0; i < 8; i++) printf("pick %lu %lu\n", i, pick(i));
    for (unsigned long i = 0; i < 8; i++) printf("letter %lu %lu\n", i, letter(i));
    for (unsigned long i = 0; i < 8; i++) printf("branchy %lu %lu\n", i, branchy(i));
    for (unsigned long i = 0; i < 8; i++) printf("add_two %lu %lu\n", i, add_two(i, 7));
    return 0;
}
"#;

struct Built {
    placed: Placed,
    expected: Vec<(String, u64, u64)>,
}

/// Builds the library, then runs a normal program against it and records
/// every answer. That program is the independent reference.
fn build() -> Option<Built> {
    let directory = std::env::temp_dir().join("hyperray_agreement");
    std::fs::create_dir_all(&directory).ok()?;
    let source = directory.join("shapes.c");
    let caller = directory.join("caller.c");
    std::fs::write(&source, SOURCE).ok()?;
    std::fs::write(&caller, CALLER).ok()?;
    let library = directory.join("libshapes.dylib");
    let program = directory.join("caller");
    run(&["-O1", "-dynamiclib", "-o"], &library, &source)?;
    link_against(&program, &caller, &library)?;
    let output = std::process::Command::new(&program)
        .arg("")
        .env("DYLD_LIBRARY_PATH", &directory)
        .output()
        .ok()?;
    let expected = read_answers(&String::from_utf8_lossy(&output.stdout));
    let placed = Placed::load(&library).ok()?;
    Some(Built { placed, expected })
}

/// Builds the caller and points it at the library it calls.
fn link_against(
    output: &std::path::Path,
    source: &std::path::Path,
    library: &std::path::Path,
) -> Option<()> {
    let status = std::process::Command::new("cc")
        .args(["-O1", "-o"])
        .arg(output)
        .arg(source)
        .arg(library)
        .status()
        .ok()?;
    status.success().then_some(())
}

fn run(flags: &[&str], output: &std::path::Path, source: &std::path::Path) -> Option<()> {
    let status = std::process::Command::new("cc")
        .args(flags)
        .arg(output)
        .arg(source)
        .status()
        .ok()?;
    status.success().then_some(())
}

fn read_answers(text: &str) -> Vec<(String, u64, u64)> {
    let mut answers = Vec::new();
    for line in text.lines() {
        let mut parts = line.split_whitespace();
        let (Some(name), Some(argument), Some(value)) = (parts.next(), parts.next(), parts.next())
        else {
            continue;
        };
        let (Ok(argument), Ok(value)) = (argument.parse(), value.parse()) else { continue };
        answers.push((name.to_string(), argument, value));
    }
    answers
}

#[test]
fn the_rechecker_agrees_with_a_normal_program() {
    let built = build().expect("the reference program must build, or this check proves nothing");
    assert!(!built.expected.is_empty(), "the reference program produced no answers");
    let mut checked = 0;
    for (name, argument, expected) in &built.expected {
        let arguments = if name == "add_two" { vec![*argument, 7] } else { vec![*argument] };
        let call = Call { arguments, buffers: Vec::new() };
        let outcome = run_placed(&built.placed, name, &call).expect("the call did not run");
        assert_eq!(
            outcome.returned, *expected,
            "{name}({argument}) disagrees with the same code called normally"
        );
        checked += 1;
    }
    println!("answers compared against a normal program: {checked}");
}
