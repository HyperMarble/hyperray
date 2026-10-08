// Purpose: with GOCACHEPROG set, go builds through that program and the
// record names it by hash. The program here is the smallest one go accepts:
// every get is a miss, every put is stored in a temp file.
// Never:   records a cache program that did not run.
package goadapter_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const fakeCache = `package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type request struct {
	ID       int64
	Command  string
	BodySize int64
}

type response struct {
	ID            int64
	KnownCommands []string ` + "`json:\",omitempty\"`" + `
	Miss          bool     ` + "`json:\",omitempty\"`" + `
	DiskPath      string   ` + "`json:\",omitempty\"`" + `
}

func main() {
	out, in := json.NewEncoder(os.Stdout), json.NewDecoder(os.Stdin)
	_ = out.Encode(response{KnownCommands: []string{"get", "put", "close"}})
	dir, _ := os.MkdirTemp("", "fakecache")
	for n := 0; ; n++ {
		var req request
		if in.Decode(&req) != nil {
			return
		}
		reply := response{ID: req.ID, Miss: req.Command == "get"}
		if req.Command == "put" {
			var body string
			data := []byte{}
			if req.BodySize > 0 {
				_ = in.Decode(&body)
				data, _ = base64.StdEncoding.DecodeString(body)
			}
			reply.DiskPath = filepath.Join(dir, fmt.Sprint(n))
			_ = os.WriteFile(reply.DiskPath, data, 0o644)
		}
		_ = out.Encode(reply)
		if req.Command == "close" {
			return
		}
	}
}
`

func TestTheCacheProgramIsHashed(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module fakecache\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(dir, "main.go"), fakeCache)
	program := filepath.Join(dir, "fakecache")
	compile := exec.Command("go", "build", "-o", program, ".")
	compile.Dir = dir
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("building the fake cache: %s", output)
	}
	t.Setenv("GOCACHEPROG", program)
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	settings := built(t, root, out, goadapter.Choice{}).Settings
	if settings.CacheProgram == nil || filepath.Base(settings.CacheProgram.File.Path) != "fakecache" {
		t.Fatalf("cache program: %+v", settings.CacheProgram)
	}
}
