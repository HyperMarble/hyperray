pub fn compose(parts: &[i32]) -> i32 {
    if parts.is_empty() {
        panic!("at least one component");
    }
    parts.iter().sum()
}

#[cfg(test)]
mod tests {
    use super::compose;

    #[test]
    fn sums_components() {
        assert_eq!(compose(&[1, 2, 3]), 6);
    }

    #[test]
    fn rejects_empty_with_message() {
        let err = std::panic::catch_unwind(|| compose(&[])).unwrap_err();
        let text = err.downcast_ref::<&str>().copied().unwrap_or("");
        assert!(text.contains("at least one component"), "got: {text}");
    }
}
