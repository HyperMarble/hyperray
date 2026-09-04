// Footprint requests remain valid only while their complete release is current.
package isla

func (request FootprintRequest) current(engine FootprintEngine) error {
	if err := request.release.current(); err != nil {
		return err
	}
	return request.release.matches(engine)
}
