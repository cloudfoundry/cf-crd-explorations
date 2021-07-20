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
