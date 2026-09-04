// Engine identity remains valid only while its executable content is unchanged.
// This comparison must occur before each proof operation.
package isla

func (engine Engine) current() error {
	return toolCurrent(engine.identity)
}

func (engine FootprintEngine) current() error {
	return toolCurrent(engine.identity)
}

func toolCurrent(identity ToolIdentity) error {
	if identity.Path == "" || identity.Digest == "" {
		return engineError(ToolIdentityFail, "engine", "unidentified tool")
	}
	digest, err := fileDigest(identity.Path)
	if err != nil {
		return engineError(ToolChanged, identity.Path, err.Error())
	}
	if digest != identity.Digest {
		return engineError(ToolChanged, identity.Path, digest)
	}
	return nil
}
