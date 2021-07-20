package filters

// queryParameterMatches is for checking if input value is not null and present in the values
func queryParameterMatches(values []string, input string) bool {
	// If map did not contain value, filter should pass through
	if values == nil {
		return true
	}
	if !contains(values, input) {
		return false
	}
	return true
}

// contains checks if the given string exists in the list
func contains(vs []string, input string) bool {
	return index(vs, input) != -1
}

// index returns the index of a given string in the provided list vs, or -1 if not present
func index(vs []string, input string) int {
	for i, v := range vs {
		if v == input {
			return i
		}
	}
	return -1
}
