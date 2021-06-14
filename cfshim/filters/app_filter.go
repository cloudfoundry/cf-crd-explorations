package filters

import (
	"fmt"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
)

type AppFilter struct {
	QueryParameters map[string][]string
}

func (a *AppFilter) Filter(input interface{}) bool {

	app, ok := input.(*appsv1alpha1.App)
	if !ok {
		fmt.Printf("Error, could not cast filter input to app\n")
		return false
	}
	//fmt.Printf("AppFilter cast to App successful: %v", *app)

	// Take the URL input list and compare to the field in the App K8s CR Object
	if !queryParameterMatches(a.QueryParameters["guids"], app.ObjectMeta.Name) {
		return false
	}
	if !queryParameterMatches(a.QueryParameters["names"], app.Spec.Name) {
		return false
	}
	if !queryParameterMatches(a.QueryParameters["stacks"], app.Spec.Lifecycle.Data.Stack) {
		return false
	}

	// Match the first lifecycle type if provided
	if val, ok := a.QueryParameters["lifecycle_type"]; ok {
		if val[0] != string(app.Spec.Type) {
			return false
		}
	}
	return true
}

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
