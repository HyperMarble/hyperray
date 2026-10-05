# Contributing to hyperray

First of all, thanks for contributing to this repository! Bug reports,
fixes, ideas and new features are all welcome.

hyperray is maintained by a small team that doesn't check in every day. If
you don't hear back right away, don't worry. We'll get to it, usually
within a few days.

## Getting set up

You'll need Rust (via `rustup`), Go 1.25+, Python 3, and `uv`. Then, once
after cloning:

```sh
uv tool install pre-commit
pre-commit install
```

Now every `git commit` runs our formatting and lint checks on the files you
changed. If one fails, it tells you what to fix. CI runs the same checks on
every pull request.

## Tests

Add a test with your change. Before opening a pull request, run the tests
for what you touched: `go test ./...` for Go, and `cargo test` inside any
Rust crate you changed.

## Issues and pull requests

- Found a bug? Open an issue with the exact input and what you got.
- Planning something big? Open an issue first so we can agree on the
  approach before you write it.
- Keep pull requests small and on one topic, and make sure CI is green.
- No code golf. Our checks keep files and functions short, but the goal is
  code that's easy to read, not code squeezed onto fewer lines. If a check
  says something is too long, split it into clear pieces. Pull requests
  that cram code to pass the checks will be closed.

## Commit messages

We use `type(scope): short summary`, for example
`fix(loader): reject a section that ends past the file`. Types are `feat`,
`fix`, `refactor`, `docs`, `test` and `chore`.

## Using AI tools

AI tools are welcome. They're great for moving faster. But the code still
has to be right, and it's yours once you submit it:

- **Read every line.** You should be able to explain why each line is
  there and why it's correct.
- **Test it yourself.** Passing CI is the minimum, not proof it's right.
- **Say what you used** in the pull request description.

Pull requests that are fully AI-generated, where the author can't explain
the changes, will be closed.

## License

By contributing, you agree that your work is licensed under the
[MIT License](LICENSE).
