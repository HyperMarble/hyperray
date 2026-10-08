// Purpose: records the go tool's own settings for this build, as `go env`
// lists them: the tool's complete list, nothing chosen by hand.
// Never:   writes a secret's value: a name that looks like a credential, or
// a value that carries a login, is recorded as set with the value
// hidden.
package tool

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/HyperMarble/hyperray/language/go/record"
)

// Words that mark a setting as a secret.
var secretWords = []string{"TOKEN", "SECRET", "PASSWORD", "CREDENTIAL", "KEY", "AUTH"}

// GoEnvironment is every `go env` setting in effect inside root, sorted
// by name. Running inside root lets the go tool honour the module's own
// `toolchain` line and any GOFLAGS the environment sets.
func GoEnvironment(root string) ([]record.EnvVar, error) {
	text, err := Printed(root, "go", "env", "-json")
	if err != nil {
		return nil, err
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(text), &values); err != nil {
		return nil, record.Unreadable("go env -json", err)
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	found := []record.EnvVar{}
	for _, name := range names {
		found = append(found, record.EnvVar{Name: name, Value: shown(name, values[name])})
	}
	return found, nil
}

// shown is the value to record, or nil when it must stay hidden.
func shown(name, value string) *string {
	if isSecret(name) || carriesLogin(value) {
		return nil
	}
	return &value
}

func isSecret(name string) bool {
	for _, word := range secretWords {
		if strings.Contains(name, word) {
			return true
		}
	}
	return false
}

// carriesLogin is true for a URL of the form scheme://user:pass@host.
func carriesLogin(value string) bool {
	return strings.Contains(value, "://") && strings.Contains(value, "@")
}

// EnvValue is the value of one recorded setting, or "" when hidden or absent.
func EnvValue(environment []record.EnvVar, name string) string {
	for _, variable := range environment {
		if variable.Name == name && variable.Value != nil {
			return *variable.Value
		}
	}
	return ""
}
