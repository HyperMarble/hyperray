// Branch frames each own a copy of their common prefix address.
// An explicit work list avoids recursion over tool-controlled trace depth.
package isla

type semanticTreeFrame struct {
	tree    string
	address semanticTreeAddress
	first   semanticTreeAddress
}

func semanticTreeCases(tree string, address semanticTreeAddress, first semanticTreeAddress, final bool) ([]semanticTreeFrame, error) {
	parts, err := semanticTreeParts(tree)
	if err != nil {
		return nil, err
	}
	if !final || len(parts) < 3 || len(parts[1]) < 2 || parts[1][0] != '"' || parts[1][len(parts[1])-1] != '"' {
		return nil, semanticProtocolError("invalid trace cases")
	}
	frames := make([]semanticTreeFrame, 0, len(parts)-2)
	for _, child := range parts[2:] {
		frames = append(frames, semanticTreeFrame{tree: child, address: address, first: first})
	}
	return frames, nil
}
