// Footprint requests remain valid only while both model inputs are unchanged.
package isla

func (request FootprintRequest) current() error {
	if err := request.architecture.current(); err != nil {
		return err
	}
	return request.configuration.current()
}
