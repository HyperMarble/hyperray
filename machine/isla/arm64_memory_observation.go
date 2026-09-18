// ARM64 memory observations name checked byte ranges for native assertions.
// They must not initialize memory or add execution effects.
package isla

// MemoryObservation names one concrete byte range in the accepted image.
type MemoryObservation struct {
	Name    string
	Address uint64
	Bytes   uint32
}

func copyMemoryObservations(values []MemoryObservation) []MemoryObservation {
	return append([]MemoryObservation(nil), values...)
}
