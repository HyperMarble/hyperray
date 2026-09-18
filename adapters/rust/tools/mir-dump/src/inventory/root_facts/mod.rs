// Root-fact extraction keeps compiler ABI tags and unresolved lowering details.
// It never converts compiler facts into valid machine roots.

mod argument;
mod scalar;
mod type_class;

use hyperray_rust::mir::RootFacts;
use rustc_public::abi::FnAbi;
use rustc_public::mir::mono::Instance;
use rustc_public::target::{Endian, MachineInfo};

pub(super) const VERSION: u32 = 1;

pub(super) fn from_abi(instance: &Instance, abi: &FnAbi, target: &str) -> RootFacts {
    let machine = MachineInfo::target();
    RootFacts {
        version: VERSION,
        target: target.to_string(),
        endian: endian(machine.endian),
        pointer_bits: machine.pointer_width.bits(),
        convention: format!("{:?}", abi.conv),
        fixed_count: abi.fixed_count,
        c_variadic: abi.c_variadic,
        requires_caller_location: instance.requires_caller_location(),
        args: abi.args.iter().map(argument::from_abi).collect(),
        ret: argument::from_abi(&abi.ret),
    }
}

fn endian(value: Endian) -> String {
    match value {
        Endian::Little => "little".to_string(),
        Endian::Big => "big".to_string(),
    }
}
