// Translation collection binds each JIB origin to one lowerer decision.
// It rejects implicit erasure and shared circuit-region ownership.
package jibcatalog

func collectTranslations(
	values []Translation,
	origins map[string]struct{},
) (map[string]struct{}, map[string]struct{}, error) {
	translated := make(map[string]struct{}, len(values))
	regions := make(map[string]struct{})
	for _, value := range values {
		if err := translationFieldsError(value); err != nil {
			return nil, nil, err
		}
		if _, exists := origins[value.OriginID]; !exists {
			return nil, nil, catalogError("unknown_origin", value.OriginID)
		}
		if _, exists := translated[value.OriginID]; exists {
			return nil, nil, catalogError("duplicate_translation", value.OriginID)
		}
		if err := dispositionError(value); err != nil {
			return nil, nil, err
		}
		if err := addRegionMappings(regions, value); err != nil {
			return nil, nil, err
		}
		translated[value.OriginID] = struct{}{}
	}
	return translated, regions, nil
}

func translationFieldsError(value Translation) error {
	fields := []struct{ name, content string }{
		{"origin_id", value.OriginID},
		{"rule_id", value.RuleID},
		{"proof_obligation_id", value.ProofObligationID},
	}
	for _, field := range fields {
		if field.content == "" {
			return catalogError("empty_translation_field", field.name)
		}
	}
	return nil
}
