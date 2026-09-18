// One instruction's semantics, taken from the already-loaded engine. The
// engine is not reloaded and no process is started.
use crate::engine::Engine;
use isla_lib::bitvector::b129::B129;
use isla_lib::bitvector::BV;
use isla_lib::executor::{self, LocalFrame, TaskState};
use isla_lib::ir::{Name, Val};
use isla_lib::smt::{self, Event, Solver};
use isla_lib::zencode;

/// The events one instruction produced.
pub struct Footprint {
    pub traces: usize,
    pub events: usize,
}

/// Runs the model's footprint function over `opcode`.
///
/// `opcode` is the instruction's bytes in memory order. A model that cannot
/// decode them is a failure, not an empty footprint.
pub fn trace(engine: &Engine, opcode: &[u8]) -> Result<Footprint, String> {
    let encoding = little_endian_word(opcode)?;
    let shared_state = &engine.ready().shared_state;
    let function = shared_state
        .symtab
        .get(&zencode::encode("isla_footprint"))
        .ok_or("model has no isla_footprint function")?;
    let (args, return_type, body) = shared_state
        .functions
        .get(&function)
        .ok_or("isla_footprint is declared but has no body")?;

    let context = smt::Context::new(smt::Config::new());
    let mut solver = Solver::<B129>::new(&context);
    assume_initial_registers(engine, &mut solver);
    let checkpoint = smt::checkpoint(&mut solver);

    let state = TaskState::new();
    let opcode_value = Val::Bits(encoding);
    let task = LocalFrame::new(function, args, return_type, Some(&[opcode_value]), body)
        .add_lets(&engine.ready().lets)
        .add_regs(&engine.ready().regs)
        .task_with_checkpoint(executor::TaskId::fresh(), &state, checkpoint);

    crate::trace_queue::drain(task, shared_state)
}

fn little_endian_word(opcode: &[u8]) -> Result<B129, String> {
    let bytes: [u8; 4] = opcode
        .try_into()
        .map_err(|_| format!("opcode is {} bytes, expected 4", opcode.len()))?;
    Ok(B129::from_u32(u32::from_le_bytes(bytes)))
}

fn assume_initial_registers(engine: &Engine, solver: &mut Solver<B129>) {
    let mut named: Vec<(&Name, _)> = engine.ready().regs.iter().collect();
    named.sort_by_key(|(name, _)| *name);
    for (name, register) in named {
        if let Some(value) = register.read_last_if_initialized() {
            solver.add_event(Event::AssumeReg(*name, vec![], value.clone()))
        }
    }
}
