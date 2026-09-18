// Tree inventory inherits prefix state independently for each child trace.
// Sibling traces cannot supply one another's instruction addresses.
package isla

type semanticTreeAddress struct {
	value uint64
	found bool
}

type semanticTreeInventory struct {
	instructions   map[SemanticInstruction]struct{}
	encodings      map[string]struct{}
	count          uint64
	entryAddresses map[uint64]struct{}
}

func (inventory *semanticTreeInventory) readTrace(tree string, address semanticTreeAddress) error {
	pending := []semanticTreeFrame{{tree: tree, address: address, first: semanticTreeAddress{}}}
	for len(pending) != 0 {
		frame := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		children, err := inventory.readPrefix(frame)
		if err != nil {
			return err
		}
		pending = append(pending, children...)
	}
	return nil
}

func (inventory *semanticTreeInventory) readPrefix(frame semanticTreeFrame) ([]semanticTreeFrame, error) {
	parts, err := semanticTreeParts(frame.tree)
	if err != nil {
		return nil, err
	}
	if len(parts) < 2 || parts[0] != "trace" {
		return nil, semanticProtocolError("expected nonempty trace tree")
	}
	for index, event := range parts[1:] {
		kind, err := traceEventHead(event)
		if err != nil {
			return nil, err
		}
		if kind == "cases" {
			return semanticTreeCases(event, frame.address, frame.first, index == len(parts)-2)
		}
		frame.address, frame.first, err = inventory.readEvent(event, kind, frame.address, frame.first)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}
