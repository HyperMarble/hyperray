package tests

import (
	"os"
	"testing"

	"github.com/HyperMarble/ray/internal/oracle"
)

// testOraclePython locates the patched touchstone-prover venv's python3 for
// tests: RAY_ORACLE_PYTHON env var. Skips the calling test if unset -- the
// patched venv isn't bundled yet (v0.1.0), built on demand via
// third_party/touchstone-patch/build.sh.
func testOraclePython(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("RAY_ORACLE_PYTHON"); p != "" {
		return p
	}
	t.Skip("RAY_ORACLE_PYTHON not set; run third_party/touchstone-patch/build.sh and point it at <venv>/bin/python3")
	return ""
}

func TestOracle_PlainProof(t *testing.T) {
	python := testOraclePython(t)
	src := "def clamp(x: int, lo: int, hi: int):\n" +
		"    if x < lo:\n        return lo\n" +
		"    if x > hi:\n        return hi\n    return x\n"
	v, err := oracle.Prove(python, src, "result >= lo and result <= hi", "lo <= hi")
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	if v.Status != "PROVED" {
		t.Fatalf("got status %q, want PROVED: %+v", v.Status, v)
	}
}

func TestOracle_PlainRefutation(t *testing.T) {
	python := testOraclePython(t)
	src := "def identity(x: int):\n    return x\n"
	v, err := oracle.Prove(python, src, "result == 0", "True")
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	if v.Status != "REFUTED" {
		t.Fatalf("got status %q, want REFUTED: %+v", v.Status, v)
	}
	if v.Counterexample == "" {
		t.Error("REFUTED verdict missing a counterexample")
	}
}

// TestOracle_AutoAnnotate_ResolvesFromRealStub is the regression test for
// the auto_annotate pre-pass: zero manual annotation anywhere in src, yet
// the unmodeled math.isnan call still gets narrowed and proves, because
// mypy resolves its real declared return type from the stdlib's own
// bundled stub. This is the "can't we just detect it already" case --
// auto_annotate is on by default (oracle.Prove doesn't expose a way to
// turn it off, so this also confirms the default path).
func TestOracle_AutoAnnotate_ResolvesFromRealStub(t *testing.T) {
	python := testOraclePython(t)
	src := "def f(x):\n" +
		"    import math\n" +
		"    r = math.isnan(x)\n" + // deliberately unannotated
		"    return r\n"
	v, err := oracle.Prove(python, src, "result == True or result == False", "True")
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	if v.Status != "PROVED" {
		t.Fatalf("got status %q, want PROVED (auto_annotate should have resolved math.isnan's real return type): %+v", v.Status, v)
	}
}

// TestOracle_AutoAnnotate_DeclinesWithoutStub confirms the honest boundary:
// when no stub is available to resolve a call's real return type (here,
// pandas-stubs genuinely doesn't model Index.any() -- confirmed against
// real mypy AND pyright, not a tooling quirk), auto_annotate leaves the
// call unannotated and the property comes back UNKNOWN rather than a
// false PROVED/REFUTED. The manual-annotation patch is what closes this
// gap when a human or AI writes the annotation directly.
func TestOracle_AutoAnnotate_DeclinesWithoutStub(t *testing.T) {
	python := testOraclePython(t)
	src := "def f(n: int):\n" +
		"    import pandas as pd\n" +
		"    idx = pd.Index(range(n)).any()\n" + // deliberately unannotated, unresolvable stub
		"    return idx\n"
	v, err := oracle.Prove(python, src, "result == True or result == False", "n >= 0")
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	if v.Status != "UNKNOWN" {
		t.Fatalf("got status %q, want UNKNOWN (auto_annotate should not have guessed a type here): %+v", v.Status, v)
	}
}

// typedNarrowingCases is the ray-typed.patch regression suite: 20 real,
// distinct dependencies pulled unbiased from the real task corpora
// (/Volumes/Hak_SSD/pluto/tasks, /Volumes/Hak_SSD/deep-swe/tasks) via their
// actual Dockerfile/requirements.txt pip installs, not hand-picked for
// looking interesting. Every call shape was verified hands-on via real
// introspection before being written here (see the session's stress-test
// scripts under touchstone-patched/), not recalled from memory.
//
// touchstone never actually imports or executes these packages -- it
// reasons over the AST only -- confirmed by these tests passing with none
// of the 20 installed in the oracle venv. What varies across cases, and
// what the patch must handle uniformly, is the call *shape*: a
// module-qualified chained method call, a plain module-function call, a
// call wrapped in a builtin (len/isinstance/bool), and -- the yaml/tomli/
// packaging cases -- a call that declines by RAISING Unsupported rather
// than by returning _Opaque, which was a real gap the first patch version
// missed (fixed by catching Unsupported around the wrapped call too).
//
// A tautology-safe property differs by declared type: bool uses the
// classic `== True or == False`; a genuinely free int uses reflexive
// `result == result`; a free float must survive IEEE-754 NaN (where
// `NaN == NaN` is false), so it uses `result == result or result !=
// result` instead -- using plain `result == result` for a float case
// looks like a tautology but isn't, and was a real test-design bug caught
// while building this table (touchstone correctly refuted it).
var typedNarrowingCases = []struct {
	name       string
	src        string
	tautology  string
	falseClaim string
	requires   string
}{
	{"jsonschema", "def check(instance):\n" +
		"    import jsonschema\n" +
		"    schema = {'type': 'string'}\n" +
		"    ok: bool = jsonschema.Draft7Validator(schema).is_valid(instance)\n" +
		"    return ok\n",
		"result == True or result == False", "result == False", "True"},

	{"pandas", "def has_any_rows(n: int):\n" +
		"    import pandas as pd\n" +
		"    idx: bool = pd.Index(range(n)).any()\n" +
		"    return idx\n",
		"result == True or result == False", "result == True", "n >= 0"},

	{"numpy", "def is_nan(x: float):\n" +
		"    import numpy as np\n" +
		"    r: bool = np.isnan(x)\n" +
		"    return r\n",
		"result == True or result == False", "result == True", "True"},

	{"xxhash", "def digest():\n" +
		"    import xxhash\n" +
		"    h: int = xxhash.xxh32(b'const').intdigest()\n" +
		"    return h\n",
		"result == result", "result == 42", "True"},

	{"pathspec", "def f(name):\n" +
		"    import pathspec\n" +
		"    r: bool = pathspec.PathSpec.from_lines('gitwildmatch', ['*.py']).match_file(name)\n" +
		"    return r\n",
		"result == True or result == False", "result == True", "True"},

	{"bs4", "def f(html):\n" +
		"    import bs4\n" +
		"    r: bool = bs4.BeautifulSoup(html, 'html.parser').a.has_attr('href')\n" +
		"    return r\n",
		"result == True or result == False", "result == False", "True"},

	{"attr", "def f():\n" +
		"    import attr\n" +
		"    r: bool = attr.has(int)\n" +
		"    return r\n",
		"result == True or result == False", "result == True", "True"},

	{"dateutil", "def f(s):\n" +
		"    import dateutil.parser\n" +
		"    r: float = dateutil.parser.isoparse(s).timestamp()\n" +
		"    return r\n",
		"result == result or result != result", "result == 0.0", "True"},

	{"tabulate", "def f(rows):\n" +
		"    import tabulate\n" +
		"    r: int = len(tabulate.tabulate(rows))\n" +
		"    return r\n",
		"result == result", "result == 999999", "True"},

	{"msgpack", "def f(d):\n" +
		"    import msgpack\n" +
		"    r: int = len(msgpack.packb(d))\n" +
		"    return r\n",
		"result == result", "result == 999999", "True"},

	{"yaml", "def f(s):\n" +
		"    import yaml\n" +
		"    ok: bool = isinstance(yaml.safe_load(s), dict)\n" +
		"    return ok\n",
		"result == True or result == False", "result == True", "True"},

	{"tomli", "def f(s):\n" +
		"    import tomli\n" +
		"    ok: bool = isinstance(tomli.loads(s), dict)\n" +
		"    return ok\n",
		"result == True or result == False", "result == True", "True"},

	{"tomli_w", "def f(d):\n" +
		"    import tomli_w\n" +
		"    r: int = len(tomli_w.dumps(d))\n" +
		"    return r\n",
		"result == result", "result == 999999", "True"},

	{"git", "def f(path):\n" +
		"    import git\n" +
		"    r: bool = git.repo.fun.is_git_dir(path)\n" +
		"    return r\n",
		"result == True or result == False", "result == True", "True"},

	{"pyflakes", "def f(path):\n" +
		"    import pyflakes.api\n" +
		"    r: int = pyflakes.api.checkPath(path)\n" +
		"    return r\n",
		"result == result", "result == -1", "True"},

	{"redis", "def f():\n" +
		"    import redis\n" +
		"    r: int = redis.Redis().dbsize()\n" +
		"    return r\n",
		"result == result", "result == -5", "True"},

	{"cachetools", "def f():\n" +
		"    import cachetools\n" +
		"    c = cachetools.LRUCache(maxsize=2)\n" +
		"    r: int = len(c)\n" +
		"    return r\n",
		"result == result", "result == 999999", "True"},

	{"toml", "def f(d):\n" +
		"    import toml\n" +
		"    r: int = len(toml.dumps(d))\n" +
		"    return r\n",
		"result == result", "result == 999999", "True"},

	{"portpicker", "def f(port: int):\n" +
		"    import portpicker\n" +
		"    r: bool = portpicker.is_port_free(port)\n" +
		"    return r\n",
		"result == True or result == False", "result == True", "True"},

	{"packaging", "def f(s):\n" +
		"    import packaging.version\n" +
		"    r: bool = bool(packaging.version.parse(s).is_prerelease)\n" +
		"    return r\n",
		"result == True or result == False", "result == True", "True"},

	// Second unbiased batch, same corpus scan continued past the first 20.
	{"hypothesis", "def f():\n" +
		"    import hypothesis.strategies as st\n" +
		"    r: int = st.integers(min_value=0, max_value=10).example()\n" +
		"    return r\n",
		"result == result", "result == 999999", "True"},

	{"graphql-core", "def f(schema, type_a, type_b):\n" +
		"    import graphql\n" +
		"    r: bool = graphql.is_type_sub_type_of(schema, type_a, type_b)\n" +
		"    return r\n",
		"result == True or result == False", "result == True", "True"},

	{"lxml", "def f(x):\n" +
		"    import lxml.etree\n" +
		"    r: bool = lxml.etree.iselement(x)\n" +
		"    return r\n",
		"result == True or result == False", "result == False", "True"},
}

func TestOracle_TypedNarrowing(t *testing.T) {
	python := testOraclePython(t)
	for _, c := range typedNarrowingCases {
		t.Run(c.name+"/tautology", func(t *testing.T) {
			v, err := oracle.Prove(python, c.src, c.tautology, c.requires)
			if err != nil {
				t.Fatalf("Prove: %v", err)
			}
			if v.Status != "PROVED" {
				t.Fatalf("got status %q, want PROVED (ray-typed patch not narrowing the unmodeled call): %+v", v.Status, v)
			}
		})
		t.Run(c.name+"/false_claim_refutes", func(t *testing.T) {
			v, err := oracle.Prove(python, c.src, c.falseClaim, c.requires)
			if err != nil {
				t.Fatalf("Prove: %v", err)
			}
			if v.Status != "REFUTED" {
				t.Fatalf("got status %q, want REFUTED (patch may be rubber-stamping): %+v", v.Status, v)
			}
		})
	}
}
