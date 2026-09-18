// Catalog errors name the rejected evidence boundary.
// They never return a partial proof object.
package compilercatalog

import "fmt"

func catalogError(format string, values ...any) error {
	return fmt.Errorf(format, values...)
}
