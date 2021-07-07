package filters

import (
	"fmt"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
)

type PackageFilter struct {
	QueryParameters map[string][]string
}

func (p *PackageFilter) Filter(input interface{}) bool {

	pk, ok := input.(*appsv1alpha1.Package)
	if !ok {
		fmt.Printf("Error, could not cast filter input to package\n")
		return false
	}
	//fmt.Printf("AppFilter cast to App successful: %v", *app)

	// Take the URL input list and compare to the field in the Package K8s CR Object
	if !queryParameterMatches(p.QueryParameters["guids"], pk.ObjectMeta.Name) {
		return false
	}

	return true
}
