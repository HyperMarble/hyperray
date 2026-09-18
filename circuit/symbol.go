// Stable private symbols isolate solver syntax from public identifiers.
// They never depend on map iteration or solver-generated names.
package circuit

import "strconv"

func quotedSymbol(name string) string {
	return "|" + name + "|"
}

func stateSymbol(name string) string {
	return quotedSymbol("state." + name)
}

func inputSymbol(name string) string {
	return quotedSymbol("input." + name)
}

func referenceNextSymbol(position int) string {
	return quotedSymbol("reference.next." + strconv.Itoa(position))
}

func candidateNextSymbol(position int) string {
	return quotedSymbol("candidate.next." + strconv.Itoa(position))
}

func referenceObservationSymbol(position int) string {
	return quotedSymbol("reference.observation." + strconv.Itoa(position))
}

func candidateObservationSymbol(position int) string {
	return quotedSymbol("candidate.observation." + strconv.Itoa(position))
}
