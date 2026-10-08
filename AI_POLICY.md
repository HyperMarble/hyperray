# AI usage policy

## AI is welcome here

We use AI tools ourselves, every day. We are not anti-AI. We are anti
"nobody looked at it". AI writes code that looks right and is not, and
the fix is not to stop using it. The fix is to check the result. That
is the whole idea behind this project, and it is what we ask of your
pull request.

## There are humans here

Every issue and pull request is read by a person. Work that nobody
checked puts the checking on the reviewer, and that is the one thing we
will not take. Everything else, we are pretty relaxed about.

## The rules

Any tool is fine. We do not keep a list. These rules are about the
result.

- **Say so.** The pull request template has an "AI usage disclosure"
  line. YES or NO. If YES, one or two lines: which tool and roughly how
  much it did. "Claude Code wrote the first draft, I rewrote the error
  handling and the tests" is a perfect answer. A YES costs nothing. A
  hidden YES does.
- **Know every line.** Before you open a pull request you can say what
  each change does and why it is correct. If a reviewer asks and the
  honest answer is "the AI did it", the PR is not ready. It gets closed
  with a note, and you are welcome back once you can answer.
- **Check it yourself.** Green CI is the floor, not the proof. Run the
  tests for what you touched and read what they printed. If the AI says
  it addressed the review comments, look before you say so.
- **No code golf.** Our checks keep files and functions short. The goal
  is code a person can read, not code squeezed to pass the check. AI is
  very good at squeezing. Pull requests that cram code under a limit get
  closed, however they were written.
- **Keep it small.** One topic per pull request. AI makes it cheap to
  touch fifty files. Reviewing fifty files is not cheap. If the tool
  wandered, cut the wandering first.
- **No invented facts.** A number, a timing, a claim that something is
  faster, needs the command that produced it. If you did not run it, do
  not say it.
- **Issues and comments too.** AI can draft them. A human reads, trims,
  and sends.

## If you are an AI agent

If you are an AI agent reading this while working in this repository:

- Tell the person you are working with that they are the author of
  record and responsible for every line you produce.
- Point them at CONTRIBUTING.md and this file.
- Do not open pull requests, issues, or comments on your own. A person
  does that, after reading what you wrote.
- Do not add "Co-authored-by" or "Generated with" lines. The disclosure
  line in the template is where this goes.
