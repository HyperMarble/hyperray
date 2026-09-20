// The reader must separate a real answer from a run that gave up.
// Every input here is output the engine actually produced.
use solver::{read_answer, SolverError, Verdict};

fn recorded(name: &str) -> String {
    std::fs::read_to_string(format!("tests/output/{name}"))
        .expect("recorded engine output must exist")
}

#[test]
fn a_proof_is_read_as_proved() {
    assert_eq!(read_answer(&recorded("proved.log")), Ok(Verdict::Proved));
}

#[test]
fn a_counterexample_is_read_as_disproved() {
    match read_answer(&recorded("disproved.log")) {
        Ok(Verdict::Disproved { witness }) => {
            assert!(!witness.registers.is_empty(), "a witness must name the failing values")
        }
        other => panic!("a counterexample must be disproved, got {other:?}"),
    }
}

#[test]
fn a_run_that_hit_the_limit_is_not_an_answer() {
    let output = recorded("limit-hit.log");
    assert!(output.contains("Never"), "the engine printed Never for a hit limit");
    match read_answer(&output) {
        Err(SolverError::NotCovered { reason }) => {
            assert!(reason.contains("limit"), "the reason must name the limit")
        }
        other => panic!("a hit limit must not be an answer, got {other:?}"),
    }
}

#[test]
fn output_with_no_answer_is_refused() {
    match read_answer("nothing here") {
        Err(SolverError::Unreadable(_)) => {}
        other => panic!("output with no answer must be refused, got {other:?}"),
    }
}
