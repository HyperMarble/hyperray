// Engine identity remains valid only while its executable content is unchanged.
// This comparison must occur before each proof operation.
package isla

func (engine Engine) current() error {
	if engine.identity.Path == "" || engine.identity.Digest == "" {
		return engineError(ToolIdentityFail, "engine", "unidentified tool")
	}
	digest, err := fileDigest(engine.identity.Path)
	if err != nil {
		return engineError(ToolChanged, engine.identity.Path, err.Error())
	}
	if digest != engine.identity.Digest {
		return engineError(ToolChanged, engine.identity.Path, digest)
	}
	return nil
}
