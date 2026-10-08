// Purpose: what the adapter does with every flag go help build names. Most
// are recorded as given; six are handled, because they pull something into
// the build from outside the project. A test reads go help build, so a
// flag missing from this table fails by name.
// Never:   refuses a flag for being a mode.
package tool

import (
	"path/filepath"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/record"
)

const recorded = "recorded in build_flags"

// flagHandling is every go build flag and what the adapter does with it.
var flagHandling = map[string]string{
	"a": recorded, "asan": recorded, "asmflags": recorded, "buildvcs": recorded, "compiler": recorded,
	"cover": recorded, "covermode": recorded, "coverpkg": recorded, "gccgoflags": recorded,
	"gcflags": recorded, "installsuffix": recorded, "json": recorded, "ldflags": recorded,
	"linkshared": recorded, "modcacherw": recorded, "msan": recorded, "n": recorded, "p": recorded,
	"pkgdir": recorded, "race": recorded, "trimpath": recorded, "v": recorded, "work": recorded, "x": recorded,
	"buildmode": "recorded, and names the kind of file built",
	"tags":      "recorded, and the tags go stamped are the record's tags",
	"pgo":       "recorded, and the profile go stamped is hashed",
	"C":         "handled: that folder becomes the project root",
	"overlay":   "handled: merged with the keep overlay, every backing file hashed",
	"modfile":   "handled: the alternate go.mod and its .sum are hashed",
	"mod":       "handled: with mod, the pin files are hashed before and after",
	"toolexec":  "handled: the wrapper program is hashed",
}

// BuildFlags is every flag the build ran with, in the order Go reads them:
// GOFLAGS from the environment first, then what was requested.
func BuildFlags(root string, choice record.Choice) ([]string, error) {
	value, err := Printed(root, "go", "env", "GOFLAGS")
	if err != nil {
		return nil, err
	}
	return append(strings.Fields(value), choice.Args()...), nil
}

// FlagValue is the value of the last -name in flags, written -name=value
// or -name value, and whether it was there at all.
func FlagValue(flags []string, name string) (string, bool) {
	value, found := "", false
	for index, flag := range flags {
		key, inline, hasInline := strings.Cut(strings.TrimLeft(flag, "-"), "=")
		if !strings.HasPrefix(flag, "-") || key != name {
			continue
		}
		found, value = true, inline
		if !hasInline && index+1 < len(flags) {
			value = flags[index+1]
		}
	}
	return value, found
}

// Relocated applies -C the way go does, before anything else: the named
// folder becomes the project root and the flag is not passed on.
func Relocated(root string, choice record.Choice) (string, record.Choice) {
	dir, found := FlagValue(choice.Flags, "C")
	if !found {
		return root, choice
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	return dir, record.Choice{Tags: choice.Tags, Flags: withoutC(choice.Flags)}
}

func withoutC(flags []string) []string {
	kept := []string{}
	for index := 0; index < len(flags); index++ {
		key, _, inline := strings.Cut(strings.TrimLeft(flags[index], "-"), "=")
		if key == "C" && strings.HasPrefix(flags[index], "-") && !inline {
			index++
			continue
		}
		if key == "C" && strings.HasPrefix(flags[index], "-") {
			continue
		}
		kept = append(kept, flags[index])
	}
	return kept
}
