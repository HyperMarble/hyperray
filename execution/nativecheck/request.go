// Requests name trusted artifacts from one native preparation.
// This boundary must not infer input limits or executable identity.
package nativecheck

import (
	"errors"
	"path/filepath"

	"github.com/HyperMarble/hyperray/execution"
)

type Request struct {
	Search           execution.Request `json:"search"`
	ReplayExecutable string            `json:"replay_executable"`
	Minimum          uint64            `json:"minimum"`
	Maximum          uint64            `json:"maximum"`
}

func (request Request) Validate() error {
	if err := request.Search.Validate(); err != nil {
		return err
	}
	if request.Minimum > request.Maximum {
		return errors.New("native interval is reversed")
	}
	if !filepath.IsAbs(request.ReplayExecutable) {
		return errors.New("native replay executable must be an absolute path")
	}
	return nil
}
