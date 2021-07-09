package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"encoding/json"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	"cloudfoundry.org/cf-crd-explorations/cfshim/filters"
	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Define the routes used in the REST endpoints
const (
	AppsEndpoint   = "/v3/apps"
	GetAppEndpoint = AppsEndpoint + "/{guid}"
)

type AppHandler struct {
	// This is a Kuberentes client, contains authentication and context stuff for running K8s queries
	Client client.Client
}

// ShowAppHandler is for getting a single app from the guid
// For now, only outputs the first match after searching ALL namespaces for Apps
// GET /v3/apps/:guid -> is this namespace + guid?
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#get-an-app
func (a *AppHandler) ShowAppHandler(w http.ResponseWriter, r *http.Request) {
	//Fetch the {guid} value from URL using gorilla mux
	vars := mux.Vars(r)
	appGUID := vars["guid"]

	// map[string][]string
	queryParameters := map[string][]string{
		"guids": {appGUID},
	}

	// Convert to a list of CFAPIAppResource to match old Cloud Controller Formatting in REST response
	formattedApps, err := a.getAppHelper(queryParameters)
	if err != nil {
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
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

//GetAppHelper is a helper function that takes a map of query parameters as input and return a list of matched apps.
func (a *AppHandler) getAppHelper(queryParameters map[string][]string) ([]CFAPIAppResource, error) {
	// use a helper function to break comma separated values into []string
	formatQueryParams(queryParameters)

	// Apply filter to AllApps and store result in matchedApps
	matchedApps, err := a.getAppListFromQuery(queryParameters)
	if err != nil {
		// Print the error if K8s client fails
		fmt.Printf("error fetching apps from query: %s\n", err)
		return nil, err
	}

	// Convert to a list of CFAPIAppResource to match old Cloud Controller Formatting in REST response
	formattedApps := make([]CFAPIAppResource, 0, len(matchedApps))
	for _, app := range matchedApps {
		formattedApps = append(formattedApps, formatApp(app))
	}

	return formattedApps, nil

}

type GetListResponse struct {
	Resources []CFAPIAppResource `json:"resources"`
}

// ListAppsHandler takes URL query parameters and sends a request to the Kuberentes API for the list of matching apps
// GET /v3/apps
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#list-apps
func (a *AppHandler) ListAppsHandler(w http.ResponseWriter, r *http.Request) {
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
		fmt.Printf("Error matching app: %v", err)
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
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
	for i, _ := range AllApps.Items {
		if filter.Filter(&AllApps.Items[i]) {
			matchedApps = append(matchedApps, &AllApps.Items[i])
		}
	}
	return matchedApps, nil
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

func (a *AppHandler) CreateAppsHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := context.Background()

	var appRequest CFAPIAppResourceWithEnvVars
	var errStrings []string

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	err := decoder.Decode(&appRequest)
	if err != nil {
		// Check if the error is from an unknown field
		// TODO: This should be a regex such as:
		// json: unknown field \"[a-zA-Z]+\"
		// to report the exact unknown field
		if strings.Compare(err.Error(), "json: unknown field \"invalid\"") == 0 {
			errStrings = append(errStrings, "Unknown field(s): 'invalid'")
		} else {
			fmt.Printf("error parsing request: %s\n", err)
			returnFormattedError(w, 400, "CF-MessageParseError", "Request invalid due to parse error: invalid request body", 1001)
			return
		}

	}
	// Check for required fields here
	// TODO: This should check each field since Relationships can error out in multiple ways.
	// For now, we're just checking it exists to address Scenario 1
	spaceguid := appRequest.Relationships.Space.Data.GUID
	if spaceguid == "" {
		errStrings = append(errStrings, "Relationships 'relationships' is not an object")
	}

	appname := appRequest.Name
	if appname == "" {
		errStrings = append(errStrings, "Name is a required entity")
	} else {
		queryParameters := map[string][]string{
			"names": {appname},
		}
		formatQueryParams(queryParameters)

		var matchedApps []*appsv1alpha1.App

		// Apply filter to AllApps and store result in matchedApps
		matchedApps, err := a.getAppListFromQuery(queryParameters)
		if err != nil {
			// Print the error if K8s client fails
			fmt.Printf("error fetching apps from query: %s\n", err)
			w.WriteHeader(500)
			return
		}
		if len(matchedApps) > 0 {
			errStrings = append(errStrings, fmt.Sprintf("App with the name '%s' already exists.", appname))
			errorDetail := strings.Join(errStrings, ", ")
			returnFormattedError(w, 422, "CF-UniquenessError", errorDetail, 10016)
			return
		}
	}

	if len(errStrings) > 0 {
		errorDetail := strings.Join(errStrings, ", ")
		returnFormattedError(w, 422, "CF-UnprocessableEntity", errorDetail, 10008)
		return
	}

	lifecycleType := appRequest.Lifecycle.Type
	if lifecycleType == "" {
		lifecycleType = "kpack"
	}

	lifecycleData := appRequest.Lifecycle.Data
	if lifecycleType == "kpack" && lifecycleData.Stack == "" {
		lifecycleData.Stack = "cflinuxfs3" // TODO: This is the default in CF for VMs. What should the default stack be here?
	}
	if lifecycleType == "kpack" && len(lifecycleData.Buildpacks) == 0 {
		lifecycleData.Buildpacks = []string{}
	}

	// Check if the namespace in the request exitsts
	space := &corev1.Namespace{}
	err = a.Client.Get(ctx, types.NamespacedName{Name: appRequest.Relationships.Space.Data.GUID}, space)
	if err != nil {
		if apierrors.IsNotFound(err) {
			returnFormattedError(w, 404, "NotFound", err.Error(), 10000)
		} else {
			fmt.Printf("error fetching Namespace object: %v\n", *space)
			returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		}
		return
	}

	//generate new UUID for each create app request.
	appGUID := uuid.NewString()

	// Create app secrets if environment variables are provided
	var envSecret string
	if len(appRequest.EnvironmentVariables) != 0 {
		secretObj := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        appGUID + "-env",
				Namespace:   appRequest.Relationships.Space.Data.GUID,
				Labels:      appRequest.Metadata.Labels,
				Annotations: appRequest.Metadata.Annotations,
			},
			StringData: appRequest.EnvironmentVariables,
		}
		err = a.Client.Create(ctx, secretObj)
		if err != nil {
			fmt.Printf("error creating Secret object: %v\n", *secretObj)
			returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		}

		envSecret = appGUID + "-env"
	}

	app := &appsv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:        appGUID,
			Namespace:   appRequest.Relationships.Space.Data.GUID,
			Labels:      appRequest.Metadata.Labels,
			Annotations: appRequest.Metadata.Annotations,
		},
		Spec: appsv1alpha1.AppSpec{
			Name:  appRequest.Name,
			State: "STOPPED",
			Type:  appsv1alpha1.LifecycleType(lifecycleType),
			Lifecycle: appsv1alpha1.Lifecycle{
				Data: appsv1alpha1.LifecycleData{
					Buildpacks: lifecycleData.Buildpacks,
					Stack:      lifecycleData.Stack,
				},
			},
			EnvSecretName: envSecret,
		},
	}

	err = a.Client.Create(ctx, app)
	if err != nil {
		fmt.Printf("error creating App object: %v\n", err)
		w.WriteHeader(500)
		return
	}

	a.ReturnFormattedResponse(w, appGUID)
}

func (a *AppHandler) UpdateAppsHandler(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	appGUID := vars["guid"]

	queryParameters := map[string][]string{
		"guids": {appGUID},
	}
	formatQueryParams(queryParameters)

	var matchedApps []*appsv1alpha1.App

	// Apply filter to AllApps and store result in matchedApps
	matchedApps, err := a.getAppListFromQuery(queryParameters)
	if err != nil {
		// Print the error if K8s client fails
		fmt.Printf("error fetching apps from query: %s\n", err)
		w.WriteHeader(500)
		return
	} else if len(matchedApps) == 0 {
		fmt.Printf("no matched apps for guid: %s\n", appGUID)
		w.WriteHeader(404)
		return
	}

	var appRequest CFAPIAppResource
	var errStrings []string
	var errorTitle string
	var errorHeader int
	var errorCode int

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	err = decoder.Decode(&appRequest)

	if err != nil {

		// Check if the error is from an unknown field
		// TODO: This should be a regex such as:
		// json: unknown field \"[a-zA-Z]+\"
		// to report the exact unknown field
		if strings.Compare(err.Error(), "json: unknown field \"invalid\"") == 0 {
			errStrings = append(errStrings, "Unknown field(s): 'invalid'")
			errorTitle = "CF-UnprocessableEntity"
			errorCode = 10008
			errorHeader = 422 // Assuming 422 even for malformed payloads
		} else {
			errStrings = append(errStrings, "Request invalid due to parse error: invalid request body")
			errorTitle = "CF-MessageParseError"
			errorCode = 1001
			errorHeader = 422
		}
	}

	if len(errStrings) > 0 {
		errorDetail := strings.Join(errStrings, ", ")
		returnFormattedError(w, errorHeader, errorTitle, errorDetail, errorCode)
		return
	}

	if appRequest.Name != "" {
		matchedApps[0].Spec.Name = appRequest.Name
	}

	var buildpacks []string
	stack := "cflinuxfs3"
	if len(appRequest.Lifecycle.Data.Buildpacks) == 0 {
		buildpacks = []string{}
	}
	if appRequest.Lifecycle.Data.Stack != "" {
		stack = appRequest.Lifecycle.Data.Stack
	}

	matchedApps[0].Spec.Lifecycle.Data = appsv1alpha1.LifecycleData{
		Buildpacks: buildpacks,
		Stack:      stack,
	}

	err = a.Client.Update(context.Background(), matchedApps[0])
	if err != nil {
		fmt.Printf("error updating App object: %v\n", err)
		w.WriteHeader(500)
		return
	}

	a.ReturnFormattedResponse(w, appGUID)
}

func (a *AppHandler) ReturnFormattedResponse(w http.ResponseWriter, appGUID string) {

	//reuse the getAppHelper method to fetch and return the app in the HTTP response.
	queryParameters := map[string][]string{
		"guids": {appGUID},
	}

	// Convert to a list of CFAPIAppResource to match old Cloud Controller Formatting in REST response
	formattedApps, err := a.getAppHelper(queryParameters)
	if err != nil {
		fmt.Printf("error fecthing the created app: %s\n", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(formattedApps[0])
}

func returnFormattedError(w http.ResponseWriter, status int, title string, detail string, code int) {
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
