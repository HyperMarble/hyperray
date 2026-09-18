// Disposition rules distinguish emitted behavior from proved erasure.
// They never permit an origin to disappear without evidence.
package jibcatalog

func dispositionError(value Translation) error {
	switch value.Disposition {
	case DispositionEmitted:
		if len(value.RegionIDs) == 0 {
			return catalogError("emitted_without_region", value.OriginID)
		}
	case DispositionErased:
		if len(value.RegionIDs) != 0 {
			return catalogError("erased_with_region", value.OriginID)
		}
	default:
		return catalogError("invalid_disposition", value.OriginID, string(value.Disposition))
	}
	return nil
}

func addRegionMappings(regions map[string]struct{}, value Translation) error {
	for _, regionID := range value.RegionIDs {
		if regionID == "" {
			return catalogError("empty_region_mapping", value.OriginID)
		}
		if _, exists := regions[regionID]; exists {
			return catalogError("duplicate_region_mapping", regionID)
		}
		regions[regionID] = struct{}{}
	}
	return nil
}
