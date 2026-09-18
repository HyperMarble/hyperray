// Running one task to completion and counting what it produced.
use isla_lib::bitvector::b129::B129;
use isla_lib::executor;
use std::sync::Arc;

use crate::footprint::Footprint;

/// Runs `task` and returns the traces and events it produced.
///
/// A task that fails is reported, never counted as zero traces.
pub fn drain<'ir>(
    task: executor::Task<'ir, '_, B129>,
    shared_state: &isla_lib::ir::SharedState<'ir, B129>,
) -> Result<Footprint, String> {
    let queue = Arc::new(crossbeam::queue::SegQueue::new());
    executor::start_multi(
        1,
        None,
        vec![task],
        shared_state,
        queue.clone(),
        &executor::trace_collector,
    )
    .map_err(|error| error.to_string())?;

    let mut traces = 0;
    let mut events = 0;
    while let Some(result) = queue.pop() {
        let (_, mut collected) = result.map_err(|error| format!("{error:?}"))?;
        traces += 1;
        events += collected.events.drain(..).count();
    }
    Ok(Footprint { traces, events })
}
