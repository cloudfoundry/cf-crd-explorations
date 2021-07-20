package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	"cloudfoundry.org/cf-crd-explorations/cfshim/filters"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReturnFormattedError(w http.ResponseWriter, status int, title string, detail string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(CFAPIErrors{
		Errors: []CFAPIError{
			{
				Title:  title,
				Detail: detail,
				Code:   code,
			},
		},
	})
}

// getAppListFromQuery takes URL query parameters and queries the K8s Client for all Apps
// builds a filter based on params and walks through, placing every match into the returned list of Apps
// returns an error if something went wrong with the K8s query
func getAppListFromQuery(c *client.Client, queryParameters map[string][]string) ([]*appsv1alpha1.App, error) {
	var filter Filter = &filters.AppFilter{
		QueryParameters: queryParameters,
	}

	// Get all the CF Apps from K8s API store in AllApps which contains Items: []App
	AllApps := &appsv1alpha1.AppList{}
	err := (*c).List(context.Background(), AllApps)
	if err != nil {
		return nil, fmt.Errorf("error fetching app: %v", err)
	}

	// Apply filter to AllApps and store result in matchedApps
	var matchedApps []*appsv1alpha1.App
	for i, _ := range AllApps.Items {
		if filter.Filter(&AllApps.Items[i]) {
			matchedApps = append(matchedApps, &AllApps.Items[i])
		}
	}
	return matchedApps, nil
}

func getPackagesListFromQuery(c *client.Client, queryParameters map[string][]string) ([]*appsv1alpha1.Package, error) {
	var filter Filter = &filters.PackageFilter{
		QueryParameters: queryParameters,
	}

	AllPackages := &appsv1alpha1.PackageList{}
	err := (*c).List(context.Background(), AllPackages)
	if err != nil {
		return nil, fmt.Errorf("error fetching package: %v", err)
	}

	// Apply filter to AllPackages and store result in matchedPackages
	var matchedPackages []*appsv1alpha1.Package
	for i, _ := range AllPackages.Items {
		if filter.Filter(&AllPackages.Items[i]) {
			matchedPackages = append(matchedPackages, &AllPackages.Items[i])
		}
	}
	return matchedPackages, nil
}

func getMatchingResources(c *client.Client, queryParameters map[string][]string, i interface{}) ([]interface{}, error) {
	var filter Filter
	var matchedPackages []interface{}

	switch i.(type) {

	case appsv1alpha1.Package:
		filter = &filters.PackageFilter{
			QueryParameters: queryParameters,
		}

		objectlist := &appsv1alpha1.PackageList{}
		err := (*c).List(context.Background(), objectlist)
		if err != nil {
			return nil, fmt.Errorf("error fetching package: %v", err)
		}
		for i, _ := range objectlist.Items {
			if filter.Filter(&objectlist.Items[i]) {
				matchedPackages = append(matchedPackages, &objectlist.Items[i])
			}
		}
	case appsv1alpha1.App:
		filter = &filters.AppFilter{
			QueryParameters: queryParameters,
		}

		objectlist := &appsv1alpha1.AppList{}
		err := (*c).List(context.Background(), objectlist)
		if err != nil {
			return nil, fmt.Errorf("error fetching package: %v", err)
		}
		for i, _ := range objectlist.Items {
			if filter.Filter(&objectlist.Items[i]) {
				matchedPackages = append(matchedPackages, &objectlist.Items[i])
			}
		}

	}
	return matchedPackages, nil
}

func getDropletListFromQuery(c *client.Client, queryParameters map[string][]string) ([]*appsv1alpha1.Droplet, error) {
	var filter Filter = &filters.DropletFilter{
		QueryParameters: queryParameters,
	}

	AllDroplets := &appsv1alpha1.DropletList{}
	err := (*c).List(context.Background(), AllDroplets)
	if err != nil {
		return nil, fmt.Errorf("error fetching app: %v", err)
	}

	// Apply filter to AllApps and store result in matchedDroplets
	var matchedDroplets []*appsv1alpha1.Droplet
	for i, _ := range AllDroplets.Items {
		if filter.Filter(&AllDroplets.Items[i]) {
			matchedDroplets = append(matchedDroplets, &AllDroplets.Items[i])
		}
	}
	return matchedDroplets, nil
}

// getBuildListFromQuery takes URL query parameters and queries the K8s Client for all Builds
// builds a filter based on params and walks through, placing every match into the returned list of Builds
// returns an error if something went wrong with the K8s query
func getBuildListFromQuery(c *client.Client, queryParameters map[string][]string) ([]*appsv1alpha1.Build, error) {
	var filter Filter = &filters.BuildFilter{
		QueryParameters: queryParameters,
	}

	// Get all the CF Apps from K8s API store in AllApps which contains Items: []App
	AllBuilds := &appsv1alpha1.BuildList{}
	err := (*c).List(context.Background(), AllBuilds)
	if err != nil {
		return nil, fmt.Errorf("error fetching app: %v", err)
	}

	// Apply filter to AllApps and store result in matchedBuilds
	var matchedBuilds []*appsv1alpha1.Build
	for i, _ := range AllBuilds.Items {
		if filter.Filter(&AllBuilds.Items[i]) {
			matchedBuilds = append(matchedBuilds, &AllBuilds.Items[i])
		}
	}
	return matchedBuilds, nil
}

// formatQueryParams takes a map of string query parameters and splits any entries with commas in them in-place
func formatQueryParams(queryParams map[string][]string) {
	for key, value := range queryParams {
		var newParamsList []string
		for _, parameter := range value {
			var commaSeparatedParamsFromValue []string = strings.Split(parameter, ",")
			newParamsList = append(newParamsList, commaSeparatedParamsFromValue...)
		}
		queryParams[key] = newParamsList
	}
}
