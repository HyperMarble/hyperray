// Purpose: Hold compiler, model, and OS facts used by contract validation.
// Never: Encode one source language, ABI, register set, or copied operation list.
// In: Facts read by adapters from compiler artifacts and installed model registries.
// Out: Public values that any adapter can construct.
// Fails: These values do not infer or repair missing facts.

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ValueFact {
    pub sort: String,
    pub location: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct FunctionFact {
    pub name: String,
    pub arguments: Vec<ValueFact>,
    pub result: Option<ValueFact>,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct OperationFact {
    pub name: String,
    pub arguments: Vec<String>,
    pub result: String,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct LiteralFact {
    pub name: String,
    pub sort: String,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct OsOperationFact {
    pub interface: String,
    pub operation: String,
    pub target: String,
    pub model: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ValidationFacts {
    pub target: String,
    pub functions: Vec<FunctionFact>,
    pub logic: String,
    pub address_sort: String,
    pub size_sort: String,
    pub memory_sort: String,
    pub operations: Vec<OperationFact>,
    pub literals: Vec<LiteralFact>,
    pub os_operations: Vec<OsOperationFact>,
}
