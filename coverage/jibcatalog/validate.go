// Validation requires exact JIB-origin and circuit-region ownership.
// It never authorizes a proof from a partial catalog.
package jibcatalog

func Validate(catalog Catalog) (Report, error) {
	origins, err := collectOrigins(catalog.Origins)
	if err != nil {
		return Report{}, err
	}
	if len(origins) == 0 {
		return Report{}, catalogError("empty_origin_catalog")
	}
	regions, err := collectRegions(catalog.Regions)
	if err != nil {
		return Report{}, err
	}
	translated, mappedRegions, err := collectTranslations(catalog.Translations, origins)
	if err != nil {
		return Report{}, err
	}
	if err := requireExact(origins, translated, "missing_translation", "extra_translation"); err != nil {
		return Report{}, err
	}
	if err := requireExact(regions, mappedRegions, "missing_region_mapping", "extra_region_mapping"); err != nil {
		return Report{}, err
	}
	return Report{
		Complete: true, Origins: len(origins),
		Translations: len(translated), Regions: len(regions),
	}, nil
}
