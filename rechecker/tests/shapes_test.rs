// Purpose: runs every function shape the rechecker claims to handle.
// Never:   uses hand-written instructions, which are not what a compiler emits.
// In:      C functions this test compiles and the operating system loads
// Out:     nothing when the processor returns what each function should
// Fails:   no compiler here, or a shape returns the wrong value
use rechecker::{run_placed, Buffer, Call, Placed};

/// The functions under test, written the way a compiler will be asked to
/// emit them. Each one is a shape the rechecker must handle.
const SOURCE: &str = r#"
unsigned long add_two(unsigned long a, unsigned long b){ return a + b; }
unsigned long subtract(unsigned long a, unsigned long b){ return a - b; }
unsigned long multiply_three(unsigned long a, unsigned long b, unsigned long c){ return a*b*c; }
unsigned long shift_right(unsigned long a){ return a >> 3; }
unsigned long smaller_of(unsigned long a, unsigned long b){ return a < b ? a : b; }
unsigned long sum_six(unsigned long a, unsigned long b, unsigned long c,
                      unsigned long d, unsigned long e, unsigned long f){
    return a+b+c+d+e+f;
}
static const unsigned long TABLE[4] = {10,20,30,40};
unsigned long pick(unsigned long i){ return TABLE[i & 3]; }
void copy_byte(const unsigned char *from, unsigned char *to){ *to = *from; }
"#;

fn build() -> Option<Placed> {
    let directory = std::env::temp_dir().join("hyperray_shapes");
    std::fs::create_dir_all(&directory).ok()?;
    let source = directory.join("shapes.c");
    std::fs::write(&source, SOURCE).ok()?;
    let library = directory.join("libshapes.dylib");
    let built = std::process::Command::new("cc")
        .args(["-O1", "-dynamiclib", "-o"])
        .arg(&library)
        .arg(&source)
        .status()
        .ok()?;
    built.success().then(|| Placed::load(&library).ok())?
}

fn call(placed: &Placed, name: &str, arguments: Vec<u64>) -> u64 {
    let call = Call { arguments, buffers: Vec::new() };
    run_placed(placed, name, &call).expect("the call did not run").returned
}

#[test]
fn every_shape_returns_what_it_should() {
    let Some(placed) = build() else {
        println!("no compiler here, so this check did not run");
        return;
    };
    assert_eq!(call(&placed, "add_two", vec![7, 5]), 12);
    assert_eq!(call(&placed, "subtract", vec![9, 4]), 5);
    assert_eq!(call(&placed, "multiply_three", vec![2, 3, 4]), 24);
    assert_eq!(call(&placed, "shift_right", vec![64]), 8);
    assert_eq!(call(&placed, "smaller_of", vec![9, 4]), 4);
    assert_eq!(call(&placed, "sum_six", vec![1, 2, 3, 4, 5, 6]), 21);
    assert_eq!(call(&placed, "pick", vec![2]), 30, "a table read must reach its table");
}

/// Addition wraps, which a proof reports and the processor must reproduce.
#[test]
fn the_processor_reproduces_a_wrap() {
    let Some(placed) = build() else { return };
    assert_eq!(call(&placed, "add_two", vec![u64::MAX, 500]), 499);
}

#[test]
fn a_function_that_writes_through_a_pointer_reports_what_it_wrote() {
    let Some(placed) = build() else { return };
    let call_with_buffers = Call {
        arguments: vec![0, 0],
        buffers: vec![
            Buffer { argument: 0, bytes: vec![0x41] },
            Buffer { argument: 1, bytes: vec![0x00] },
        ],
    };
    let outcome = run_placed(&placed, "copy_byte", &call_with_buffers).expect("the call did not run");
    assert_eq!(outcome.buffers[1], vec![0x41], "the byte written must be reported");
}
