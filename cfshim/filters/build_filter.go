package filters

import (
	"fmt"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
)

type BuildFilter struct {
	QueryParameters map[string][]string
}

func (a *BuildFilter) Filter(input interface{}) bool {

	build, ok := input.(*appsv1alpha1.Build)
	if !ok {
		fmt.Printf("Error, could not cast filter input to build\n")
		return false
	}
	//fmt.Printf("BuildFilter cast to Build successful: %v", *build)

	// Take the URL input list and compare to the field in the Build K8s CR Object
	if !queryParameterMatches(a.QueryParameters["guids"], build.ObjectMeta.Name) {
		return false
	}

	// Match the first lifecycle type if provided
	if val, ok := a.QueryParameters["lifecycle_type"]; ok {
		if val[0] != string(build.Spec.Type) {
			return false
		}
	}
	return true
}
