// Purpose: Enforce the documented `.hray` expression subset on parsed SMT terms.
// Never: Reparse text, duplicate SMT typing, or accept extra SMT term forms.
// In: One parsed expression and its owning section index.
// Out: Confirmation that every node belongs to the `.hray` grammar.
// Fails: A literal, variable, identifier, or term form is outside the language.

use crate::{Expression, ValidationFailure};
use yaspar_ir::ast::alg::{Constant, Term as ExpressionKind};
use yaspar_ir::untyped::{Constant as UntypedConstant, QualifiedIdentifier};

use super::numbered_name;

pub(crate) fn check(index: usize, root: &Expression) -> Result<(), ValidationFailure> {
    let mut pending = vec![root];
    while let Some(expression) = pending.pop() {
        check_node(index, expression)?;
        pending.extend((***expression).sub_terms());
    }
    Ok(())
}

fn check_node(index: usize, expression: &Expression) -> Result<(), ValidationFailure> {
    match &***expression {
        ExpressionKind::Constant(value, _) => check_constant(index, value),
        ExpressionKind::Global(identifier, _) => check_global(index, identifier),
        ExpressionKind::Local(variable) => check_variable(index, variable.symbol.as_str()),
        ExpressionKind::App(_, arguments, _) if arguments.is_empty() => {
            Err(rejected(index, "operation requires an argument"))
        }
        ExpressionKind::App(_, _, _) => Ok(()),
        ExpressionKind::Exists(bindings, _) | ExpressionKind::Forall(bindings, _) => {
            for binding in bindings {
                check_variable(index, binding.0.as_str())?;
            }
            Ok(())
        }
        ExpressionKind::Eq(_, _)
        | ExpressionKind::Distinct(_)
        | ExpressionKind::And(_)
        | ExpressionKind::Or(_)
        | ExpressionKind::Xor(_)
        | ExpressionKind::Implies(_, _)
        | ExpressionKind::Not(_)
        | ExpressionKind::Ite(_, _, _) => Ok(()),
        _ => Err(rejected(index, "SMT term is outside `.hray`")),
    }
}

fn check_constant(index: usize, value: &UntypedConstant) -> Result<(), ValidationFailure> {
    match value {
        Constant::Bool(_) | Constant::Binary(_, _) | Constant::Hexadecimal(_, _) => Ok(()),
        _ => Err(rejected(index, "literal is outside `.hray`")),
    }
}

fn check_global(index: usize, identifier: &QualifiedIdentifier) -> Result<(), ValidationFailure> {
    let qualified = !identifier.0.indices.is_empty() || identifier.1.is_some();
    let message = "standalone qualified identifier is not legal";
    if qualified {
        return Err(rejected(index, message));
    }
    Ok(())
}

fn check_variable(index: usize, name: &str) -> Result<(), ValidationFailure> {
    if numbered_name::has_positive_suffix(name, "value") {
        return Ok(());
    }
    Err(rejected(index, "quantified variable must be `valueN`"))
}

fn rejected(index: usize, message: &str) -> ValidationFailure {
    ValidationFailure::rejected(Some(index), message)
}
