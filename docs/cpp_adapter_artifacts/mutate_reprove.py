#!/usr/bin/env python3
"""mutate_reprove.py — Hyperray stage-4 ADEQUACY for C++.

Mutates the SOURCE and re-runs the PROOF (ESBMC), not the tests.

  proof still SUCCESSFUL on a mutant -> mutant LIVE  -> spec too weak
  proof FAILED on a mutant           -> mutant KILLED -> spec row does work

Usage: mutate_reprove.py <file.cpp> [esbmc flags...]
"""
import sys, re, subprocess, tempfile, os, time, json

SRC = sys.argv[1]
FLAGS = sys.argv[2:] or ["--overflow-check"]
orig = open(SRC).read()

# Mutation operators. Kept deliberately small and syntactic, in the style of
# mull / mutate++: arithmetic, relational, boundary, constant.
OPS = [
    # (regex, replacement, label)
    (r"(?<![<>=!+\-*/])\+(?!\+|=)", "-", "AOR + -> -"),
    (r"(?<![<>=!+\-*/])-(?!-|=|>)", "+", "AOR - -> +"),
    (r"<=", "<",  "ROR <= -> <"),
    (r"(?<![<=])<(?!=)", "<=", "ROR < -> <="),
    (r">=", ">",  "ROR >= -> >"),
    (r"==", "!=", "ROR == -> !="),
    (r"\b0\b", "1", "CR 0 -> 1"),
    (r"\b1\b", "0", "CR 1 -> 0"),
]

def run_esbmc(path):
    t0 = time.time()
    p = subprocess.run(["esbmc", path] + FLAGS + ["--timeout", "30s"],
                       capture_output=True, text=True)
    dt = time.time() - t0
    out = p.stdout + p.stderr
    if "VERIFICATION SUCCESSFUL" in out:   return "SUCCESSFUL", dt
    if "VERIFICATION FAILED" in out:       return "FAILED", dt
    if "PARSING ERROR" in out or "CONVERSION ERROR" in out: return "INVALID", dt
    if "Timed out" in out:                 return "TIMEOUT", dt
    return "UNKNOWN", dt

base_verdict, base_t = run_esbmc(SRC)
print(f"baseline: {base_verdict} ({base_t:.2f}s)  flags={' '.join(FLAGS)}")
if base_verdict != "SUCCESSFUL":
    print("NOTE: baseline is not SUCCESSFUL; live/killed reading below is only")
    print("      meaningful for a passing proof. Reporting anyway.")

results = []
mid = 0
lines = orig.split("\n")
for ln, line in enumerate(lines, 1):
    if line.strip().startswith("//") or line.strip().startswith("#"):
        continue
    if "__ESBMC" in line:          # never mutate the specification itself
        continue
    for pat, rep, label in OPS:
        for m in re.finditer(pat, line):
            mid += 1
            newline = line[:m.start()] + rep + line[m.end():]
            mutated = "\n".join(lines[:ln-1] + [newline] + lines[ln:])
            fd, path = tempfile.mkstemp(suffix=".cpp", dir="/tmp/cppadapter")
            os.write(fd, mutated.encode()); os.close(fd)
            verdict, dt = run_esbmc(path)
            os.unlink(path)
            status = {"FAILED": "KILLED", "SUCCESSFUL": "LIVE"}.get(verdict, verdict)
            results.append({"id": mid, "line": ln, "op": label,
                            "before": line.strip()[:60],
                            "after": newline.strip()[:60],
                            "verdict": verdict, "status": status, "time": round(dt,2)})

killed  = [r for r in results if r["status"] == "KILLED"]
live    = [r for r in results if r["status"] == "LIVE"]
invalid = [r for r in results if r["status"] not in ("KILLED", "LIVE")]
valid   = len(killed) + len(live)

print(f"\nmutants: {len(results)} generated, {valid} compiled+ran, "
      f"{len(invalid)} invalid/timeout")
if valid:
    print(f"KILLED {len(killed)}  LIVE {len(live)}  "
          f"mutation score {100.0*len(killed)/valid:.1f}%")
print("\n--- LIVE mutants (proof still passed => spec too weak here) ---")
for r in live:
    print(f"  L{r['line']:<3} {r['op']:<16} {r['before']}  =>  {r['after']}")
json.dump(results, open(SRC + ".mutants.json", "w"), indent=1)
print(f"\nfull results: {SRC}.mutants.json")
