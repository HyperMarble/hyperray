// Origin validation binds each used kind to a visited JIB node.
// It never accepts a kind list that has no occurrence evidence.
package main

import "fmt"

type origin struct {
	ID       string `json:"origin_id"`
	Category string `json:"category"`
	Kind     string `json:"kind"`
}

func validateOrigins(value catalog) error {
	if len(value.Origins) == 0 {
		return fmt.Errorf("JIB origin catalog is empty")
	}
	allowed := allowedOriginKinds(value)
	seenIDs := make(map[string]struct{}, len(value.Origins))
	seenKinds := make(map[string]struct{})
	for _, current := range value.Origins {
		if err := validateOrigin(current, allowed, seenIDs); err != nil {
			return err
		}
		seenIDs[current.ID] = struct{}{}
		seenKinds[originKindKey(current.Category, current.Kind)] = struct{}{}
	}
	for required := range allowed {
		if _, exists := seenKinds[required]; !exists {
			return fmt.Errorf("JIB kind %q has no origin", required)
		}
	}
	return nil
}

func validateOrigin(value origin, allowed map[string]struct{}, seen map[string]struct{}) error {
	if value.ID == "" || value.Category == "" || value.Kind == "" {
		return fmt.Errorf("JIB origin has an empty field")
	}
	if _, exists := seen[value.ID]; exists {
		return fmt.Errorf("JIB origin %q is duplicated", value.ID)
	}
	key := originKindKey(value.Category, value.Kind)
	if _, exists := allowed[key]; !exists {
		return fmt.Errorf("JIB origin %q has unknown kind %q", value.ID, key)
	}
	return nil
}

func allowedOriginKinds(value catalog) map[string]struct{} {
	result := make(map[string]struct{})
	for _, current := range catalogCategories(value) {
		for _, kind := range current.values {
			result[originKindKey(current.key, kind)] = struct{}{}
		}
	}
	return result
}

func originKindKey(category string, kind string) string {
	return category + ":" + kind
}
