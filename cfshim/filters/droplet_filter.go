package filters

import (
	"fmt"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
)

type DropletFilter struct {
	QueryParameters map[string][]string
}

func (d *DropletFilter) Filter(input interface{}) bool {

	drp, ok := input.(*appsv1alpha1.Droplet)
	if !ok {
		fmt.Printf("Error, could not cast filter input to droplet\n")
		return false
	}

	if !queryParameterMatches(d.QueryParameters["guids"], drp.ObjectMeta.Name) {
		return false
	}

	return true
}
