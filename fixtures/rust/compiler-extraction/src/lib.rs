// Compiler extraction cases exercise different control-flow forms.
// Their presence in a dump is not a semantic coverage proof.

pub fn counted(limit: u8) -> u8 {
    let mut counter = 0;
    while counter < limit {
        counter += 1;
    }
    counter
}

pub fn iterator(values: &[u8]) -> u64 {
    values.iter().map(|value| u64::from(*value)).sum()
}

pub fn recursive(remaining: u8) -> u8 {
    if remaining == 0 {
        return 0;
    }
    recursive(remaining - 1) + 1
}

pub async fn future(value: u8) -> u8 {
    async { value }.await
}

pub fn generic<Value>(value: Value) -> Value {
    value
}
