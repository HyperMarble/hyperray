// What does a dynamically linked binary ask to be rewritten?
use object::{Object, ObjectSection};

fn main() {
    let Some(path) = std::env::args().nth(1) else { return };
    let Ok(bytes) = std::fs::read(&path) else { return };
    let Ok(file) = object::File::parse(&*bytes) else {
        println!("not parsed");
        return;
    };
    let dynamic: Vec<_> = file.dynamic_relocations().into_iter().flatten().collect();
    println!("dynamic relocations: {}", dynamic.len());
    for (address, relocation) in dynamic.iter().take(4) {
        println!("  0x{address:x}  {:?}", relocation.target());
    }
    let mut section_total = 0;
    for section in file.sections() {
        section_total += section.relocations().count();
    }
    println!("section relocations: {section_total}");
}
