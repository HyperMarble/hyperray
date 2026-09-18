// The driver links to the compiler that built it. Its runtime search path
// must not depend on the toolchain that starts it later.

use std::process::Command;

fn main() {
    println!("cargo:rerun-if-changed=build.rs");
    match sysroot() {
        Some(path) => println!("cargo:rustc-link-arg=-Wl,-rpath,{path}/lib"),
        None => println!("cargo:warning=rustc did not report its sysroot"),
    }
}

fn sysroot() -> Option<String> {
    let rustc = std::env::var("RUSTC").ok()?;
    let output = Command::new(rustc).arg("--print=sysroot").output().ok()?;
    let path = String::from_utf8(output.stdout).ok()?;
    Some(path.trim().to_string())
}
