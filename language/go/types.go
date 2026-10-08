// Purpose: the names a caller of the adapter uses, so the record package
// stays the one place they are defined.
// Never:   defines a type of its own.
package goadapter

import "github.com/HyperMarble/hyperray/language/go/record"

type (
	Choice      = record.Choice
	BuildRecord = record.BuildRecord
	BuildInfo   = record.BuildInfo
	Outcome     = record.Outcome
)
