package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"encoding/json"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	"cloudfoundry.org/cf-crd-explorations/cfshim/filters"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Define the routes used in the REST endpoints
const (
	AppsEndpoint   = "/v3/apps"
	GetAppEndpoint = AppsEndpoint + "/"
)

type AppHandler struct {
	// This is a Kuberentes client, contains authentication and context stuff for running K8s queries
	Client client.Client
}

// GetAppsHandler is for getting a single app from the guid
// For now, only outputs the first match after searching ALL namespaces for Apps
// GET /v3/apps/:guid -> is this namespace + guid?
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#get-an-app
func (a *AppHandler) GetAppsHandler(w http.ResponseWriter, r *http.Request) {
	// Remove the part of the app URL before the app guid
	// apps/{blah}
	appGUID := r.URL.Path[len(GetAppEndpoint):]
	// map[string][]string
	queryParameters := map[string][]string{
		"guids": {appGUID},
	}
	// use a helper function to break comma separated values into []string
	formatQueryParams(queryParameters)

	// Apply filter to AllApps and store result in matchedApps
	matchedApps, err := a.getAppListFromQuery(queryParameters)
	if err != nil {
		// Print the error if K8s client fails
		w.WriteHeader(500)
		fmt.Fprintf(w, "%v", err)
		return
	}

	// Convert to a list of CFAPIAppResource to match old Cloud Controller Formatting in REST response
	formattedApps := make([]CFAPIAppResource, 0, len(matchedApps))
	for _, app := range matchedApps {
		formattedApps = append(formattedApps, formatApp(app))
	}

	if len(formattedApps) < 1 {
		// If no matches for the GUID, just return a 404
		w.WriteHeader(404)
		return
	}
	// Write MatchedApps to http ResponseWriter
	w.Header().Set("Content-Type", "application/json")
	// We are only printing the first element in the list for now ignoring cross-namespace guid collisions
	json.NewEncoder(w).Encode(formattedApps[0])
}

type GetListResponse struct {
	Resources []CFAPIAppResource `json:"resources"`
}

// AppGetListHandler takes URL query parameters and sends a request to the Kuberentes API for the list of matching apps
// GET /v3/apps
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#list-apps
func (a *AppHandler) AppGetListHandler(w http.ResponseWriter, r *http.Request) {
	// queryParameters comes from the URL request
	// it is a map of string to list of string
	// map[string][]string
	queryParameters := r.URL.Query()
	// use a helper function to break comma separated values into []string
	formatQueryParams(queryParameters)

	// Apply filter to AllApps and store result in matchedApps
	matchedApps, err := a.getAppListFromQuery(queryParameters)
	if err != nil {
		// Print the error if K8s client fails
		w.WriteHeader(500)
		fmt.Fprintf(w, "%v", err)
		return
	}

	// Convert to a list of CFAPIAppResource to match old Cloud Controller Formatting in REST response
	formattedApps := make([]CFAPIAppResource, 0, len(matchedApps))
	for _, app := range matchedApps {
		formattedApps = append(formattedApps, formatApp(app))
	}

	// Write MatchedApps to http ResponseWriter
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetListResponse{
		Resources: formattedApps,
	})

}

// getAppListFromQuery takes URL query parameters and queries the K8s Client for all Apps
// builds a filter based on params and walks through, placing every match into the returned list of Apps
// returns an error if something went wrong with the K8s query
func (a *AppHandler) getAppListFromQuery(queryParameters map[string][]string) ([]*appsv1alpha1.App, error) {
	var filter Filter = &filters.AppFilter{
		QueryParameters: queryParameters,
	}

	// Get all the CF Apps from K8s API store in AllApps which contains Items: []App
	AllApps := &appsv1alpha1.AppList{}
	err := a.Client.List(context.Background(), AllApps)
	if err != nil {
		return nil, fmt.Errorf("error fetching app: %v", err)
	}

	// Apply filter to AllApps and store result in matchedApps
	var matchedApps []*appsv1alpha1.App
	fmt.Printf("%v\n", matchedApps)
	for i, _ := range AllApps.Items {
		if filter.Filter(&AllApps.Items[i]) {
			matchedApps = append(matchedApps, &AllApps.Items[i])
		}
	}
	return matchedApps, nil
}

// formatQueryParams takes a map of string query parameters and splits any entires with commas in them in-place
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

type Filter interface {
	// Filter takes an object, casts it uses preset filters and returns yes/no
	Filter(interface{}) bool
}

func formatApp(app *appsv1alpha1.App) CFAPIAppResource {
	return CFAPIAppResource{
		GUID:      app.Name,
		Name:      app.Spec.Name,
		State:     string(app.Spec.State),
		CreatedAt: app.CreationTimestamp.UTC().Format(time.RFC3339),
		// TODO: Solve this- kubectl creates managedFields entry for us
		UpdatedAt: "",
		Lifecycle: CFAPIAppLifecycle{
			Type: string(app.Spec.Type),
			Data: CFAPIAppLifecycleData{
				Buildpacks: app.Spec.Lifecycle.Data.Buildpacks,
				Stack:      app.Spec.Lifecycle.Data.Stack,
			},
		},
		Relationships: CFAPIAppRelationships{
			Space: CFAPIAppRelationshipsSpace{
				Data: CFAPIAppRelationshipsSpaceData{
					GUID: app.Namespace,
				},
			},
		},
		// URL information about the server where you sub in the app GUID..
		Links: map[string]CFAPIAppLink{},
		Metadata: CFAPIMetadata{
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
}
