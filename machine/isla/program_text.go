// Program names and assertions must remain printable single-line values.
// Control characters must not change the generated artifact structure.
package isla

func plainLine(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] > 0x7e {
			return false
		}
	}
	return true
}
