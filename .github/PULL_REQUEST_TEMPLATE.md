## What this changes

One sentence.

## One logic change per commit

- [ ] Each commit makes one change
- [ ] Each commit builds on its own
- [ ] Each commit has its own test

Run this and paste the result:

```
for c in $(git rev-list origin/main..HEAD); do
  printf "%s %3s files  %s\n" "$(git rev-parse --short $c)" \
    "$(git show --name-only --format="" $c | grep -c .)" \
    "$(git log -1 --format=%s $c)"
done
```

## Evidence

Every claim names the command that produced it. Paste real output.

```
paste the test run
```

If you claim something is faster, paste both numbers.

## Can this cause a wrong PROVED?

- [ ] No, and here is why
- [ ] Yes, and here is the test that would catch it

Anything that skips work, caches a result, or reuses state belongs here.
Speed is never a reason to accept a verdict change.

## Checks

- [ ] `cargo test --release` passes
- [ ] `cargo clippy --release --all-targets` reports nothing in our code
- [ ] `cargo fmt` applied
- [ ] No file over 75 lines, no function over 40
- [ ] No `unwrap`, `expect`, `panic!`, or discarded error in `src/`

## Commit messages

- Say WHAT changed, not how
- Put measurements in the body, with the command that produced them
- No `Co-authored-by`
