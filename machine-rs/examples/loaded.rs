use hyperray_machine::region;
fn main() {
    let Some(path) = std::env::args().nth(1) else { return };
    let Ok(bytes) = std::fs::read(&path) else { return };
    let Ok(found) = region::regions(&bytes) else { return };
    for one in &found {
        let end = one.address + one.bytes.len() as u64;
        println!("0x{:x}..0x{:x}  {:?}", one.address, end, one.permission);
    }
}
