package slice

// Remove duplicates from a slice of any
func RemoveDuplicates[T comparable](slice []T, isKeepOrder bool) []T {
	uniqueMap := make(map[T]bool)
	uniqueSlice := []T{}

	for _, item := range slice {
		if _, exists := uniqueMap[item]; !exists {
			uniqueMap[item] = true
			if isKeepOrder {
				uniqueSlice = append(uniqueSlice, item)
			}
		}
	}

	if !isKeepOrder {
		for item := range uniqueMap {
			uniqueSlice = append(uniqueSlice, item)
		}
	}

	return uniqueSlice
}
