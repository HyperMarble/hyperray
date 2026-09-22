// Purpose: Hold one SMT-parsed `.hray` document.
// Never: Interpret section names, roles, types, or machine meaning.
// In: One `hray` application whose children are section applications.
// Out: Public sections and their untouched SMT values.
// Fails: These values do not perform validation.

use crate::Expression;

#[derive(Clone, Debug)]
pub struct Contract {
    pub sections: Vec<Section>,
}

#[derive(Clone, Debug)]
pub struct Section {
    pub name: String,
    pub values: Vec<Expression>,
}
