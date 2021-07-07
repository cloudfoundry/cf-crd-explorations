package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	cfappsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Define the routes used in the REST endpoints
const (
	AppsEndpoint               = "/v3/apps"
	GetAppEndpoint             = AppsEndpoint + "/{guid}"
	SetCurrentDroplet          = GetAppEndpoint + "/relationships/current_droplet"
	SetAppDesiredStateEndpoint = GetAppEndpoint + "/actions/{action}"
)

type AppHandler struct {
	// This is a Kuberentes client, contains authentication and context stuff for running K8s queries
	Client client.Client
}

// GetAppHandler is for getting a single app from the guid
// For now, only outputs the first match after searching ALL namespaces for Apps
// GET /v3/apps/:guid
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#get-an-app
func (a *AppHandler) GetAppHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch the {guid} value from URL using gorilla mux
	vars := mux.Vars(r)
	appGUID := vars["guid"]

	// map[string][]string
	queryParameters := map[string][]string{
		"guids": {appGUID},
	}

	// Use the k8s client to fetch the apps with the same metadata.name as the guid
	matchedApps, err := getAppListFromQuery(&a.Client, queryParameters)
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	if len(matchedApps) < 1 {
		// If no matches for the GUID, just return a 404
		w.WriteHeader(404)
		return
	}

	// Write MatchedApps to http ResponseWriter
	w.Header().Set("Content-Type", "application/json")
	// We are only printing the first element in the list for now ignoring cross-namespace guid collisions
	a.ReturnFormattedResponse(w, matchedApps[0])
}

type GetListResponse struct {
	Resources []CFAPIPresenterAppResource `json:"resources"`
}

// ListAppsHandler takes URL query parameters and sends a request to the Kuberentes API for the list of matching apps
// GET /v3/apps
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#list-apps
func (a *AppHandler) ListAppsHandler(w http.ResponseWriter, r *http.Request) {
	config := ctrl.GetConfigOrDie()
	config = rest.AnonymousClientConfig(config)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		config.BearerToken = strings.Split(authHeader, "bearer ")[1]
	}

	fmt.Printf("config.BearerToken = \"%s\"\n", config.BearerToken)

	var err error
	a.Client, err = client.New(config, client.Options{
		Scheme: a.Client.Scheme(),
		Mapper: a.Client.RESTMapper(),
	})
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "%v", err)
		return
	}

	// queryParameters comes from the URL request
	// it is a map of string to list of string
	// map[string][]string
	queryParameters := r.URL.Query()
	// use a helper function to break comma separated values into []string
	formatQueryParams(queryParameters)

	// Apply filter to AllApps and store result in matchedApps
	matchedApps, err := getAppListFromQuery(&a.Client, queryParameters)
	if err != nil {
		// Print the error if K8s client fails
		fmt.Printf("Error matching app: %v", err)
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	// Convert to a list of CFAPIAppResource to match old Cloud Controller Formatting in REST response
	formattedApps := make([]CFAPIPresenterAppResource, 0, len(matchedApps))
	for _, app := range matchedApps {
		formattedApps = append(formattedApps, formatAppToPresenter(app))
	}

	// Write MatchedApps to http ResponseWriter
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetListResponse{
		Resources: formattedApps,
	})
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
			ReturnFormattedError(w, 400, "CF-MessageParseError", "Request invalid due to parse error: invalid request body", 1001)
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

		var matchedApps []*cfappsv1alpha1.App

		// Apply filter to AllApps and store result in matchedApps
		matchedApps, err := getAppListFromQuery(&a.Client, queryParameters)
		if err != nil {
			// Print the error if K8s client fails
			fmt.Printf("error fetching apps from query: %s\n", err)
			w.WriteHeader(500)
			return
		}
		if len(matchedApps) > 0 {
			errStrings = append(errStrings, fmt.Sprintf("App with the name '%s' already exists.", appname))
			errorDetail := strings.Join(errStrings, ", ")
			ReturnFormattedError(w, 422, "CF-UniquenessError", errorDetail, 10016)
			return
		}
	}

	if len(errStrings) > 0 {
		errorDetail := strings.Join(errStrings, ", ")
		ReturnFormattedError(w, 422, "CF-UnprocessableEntity", errorDetail, 10008)
		return
	}

	lifecycleType := appRequest.Lifecycle.Type
	if lifecycleType == "" {
		lifecycleType = string(cfappsv1alpha1.BuildpackLifecycle)
	}

	lifecycleData := appRequest.Lifecycle.Data
	if lifecycleType == string(cfappsv1alpha1.BuildpackLifecycle) && lifecycleData.Stack == "" {
		lifecycleData.Stack = "cflinuxfs3" // TODO: This is the default in CF for VMs. What should the default stack be here?
	}
	if lifecycleType == string(cfappsv1alpha1.BuildpackLifecycle) && len(lifecycleData.Buildpacks) == 0 {
		lifecycleData.Buildpacks = []string{}
	}

	// Check if the namespace in the request exists
	space := &corev1.Namespace{}
	err = a.Client.Get(ctx, types.NamespacedName{Name: appRequest.Relationships.Space.Data.GUID}, space)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ReturnFormattedError(w, 404, "NotFound", err.Error(), 10000)
		} else {
			fmt.Printf("error fetching Namespace object: %v\n", *space)
			ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		}
		return
	}

	// generate new UUID for each create app request.
	appGUID := uuid.NewString()

	// Add labels for apps.cloudfoundry.org/appGuid: my-app-guid
	if appRequest.Metadata.Labels == nil {
		appRequest.Metadata.Labels = make(map[string]string, 1)
	}
	appRequest.Metadata.Labels["apps.cloudfoundry.org/appGuid"] = appGUID

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
			ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		}

		envSecret = appGUID + "-env"
	}

	app := &cfappsv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:        appGUID,
			Namespace:   appRequest.Relationships.Space.Data.GUID,
			Labels:      appRequest.Metadata.Labels,
			Annotations: appRequest.Metadata.Annotations,
		},
		Spec: cfappsv1alpha1.AppSpec{
			Name:         appRequest.Name,
			DesiredState: "STOPPED",
			Type:         cfappsv1alpha1.LifecycleType(lifecycleType),
			Lifecycle: cfappsv1alpha1.Lifecycle{
				Data: cfappsv1alpha1.LifecycleData{
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
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	a.ReturnFormattedResponse(w, app)
}

func (a *AppHandler) UpdateAppsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appGUID := vars["guid"]

	queryParameters := map[string][]string{
		"guids": {appGUID},
	}
	formatQueryParams(queryParameters)

	var matchedApps []*cfappsv1alpha1.App

	// Apply filter to AllApps and store result in matchedApps
	matchedApps, err := getAppListFromQuery(&a.Client, queryParameters)
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
	matchedApp := matchedApps[0]

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
		ReturnFormattedError(w, errorHeader, errorTitle, errorDetail, errorCode)
		return
	}

	if appRequest.Name != "" {
		matchedApp.Spec.Name = appRequest.Name
	}

	// If metadata is provided, overwrite metadata on existing app
	if appRequest.Metadata.Labels != nil {
		// Add label for apps.cloudfoundry.org/appGuid: my-app-guid
		appRequest.Metadata.Labels["apps.cloudfoundry.org/appGuid"] = appGUID
		matchedApp.ObjectMeta.Labels = appRequest.Metadata.Labels
	}

	// if annotations is provided, overwrite annotations on existing app
	if appRequest.Metadata.Annotations != nil {
		matchedApp.ObjectMeta.Annotations = appRequest.Metadata.Annotations
	}

	lifecycleType := appRequest.Lifecycle.Type
	if lifecycleType != "" {
		matchedApp.Spec.Type = cfappsv1alpha1.LifecycleType(lifecycleType)
	}

	buildpacks := []string{}
	stack := "cflinuxfs3"
	if len(appRequest.Lifecycle.Data.Buildpacks) != 0 {
		buildpacks = appRequest.Lifecycle.Data.Buildpacks
	}
	if appRequest.Lifecycle.Data.Stack != "" {
		stack = appRequest.Lifecycle.Data.Stack
	}

	matchedApp.Spec.Lifecycle.Data = cfappsv1alpha1.LifecycleData{
		Buildpacks: buildpacks,
		Stack:      stack,
	}

	err = a.Client.Update(context.Background(), matchedApp)
	if err != nil {
		fmt.Printf("error updating App object: %v\n", err)
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	a.ReturnFormattedResponse(w, matchedApp)
}

func (a *AppHandler) SetCurrentDroplet(w http.ResponseWriter, r *http.Request) {
	var matchedApps []*cfappsv1alpha1.App
	var dropletRequest CFAPIAppRelationshipsDroplet
	var errorMessage string
	var errorTitle string
	var errorHeader int
	var errorCode int

	vars := mux.Vars(r)
	appGUID := vars["guid"]

	queryParameters := map[string][]string{
		"guids": {appGUID},
	}
	formatQueryParams(queryParameters)

	matchedApps, err := getAppListFromQuery(&a.Client, queryParameters)
	if err != nil {
		fmt.Printf("error fetching apps from query: %s\n", err)
		errorMessage = "Error fetching app"
		errorTitle = "CF-ServerError"
		errorCode = 10001
		errorHeader = 500
		ReturnFormattedError(w, errorHeader, errorTitle, errorMessage, errorCode)
		return
	} else if len(matchedApps) == 0 {
		errorMessage = fmt.Sprintf("App with guid %s not found", appGUID)
		errorTitle = "CF-ResourceNotFound"
		errorCode = 10010
		errorHeader = 404
		ReturnFormattedError(w, errorHeader, errorTitle, errorMessage, errorCode)
		return
	}
	matchedApp := matchedApps[0]

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&dropletRequest)

	if err != nil {
		errorMessage = "Request invalid due to parse error: invalid request body"
		errorTitle = "CF-MessageParseError"
		errorCode = 1001
		errorHeader = 422
		ReturnFormattedError(w, errorHeader, errorTitle, errorMessage, errorCode)
		return
	}

	queryParameters = map[string][]string{
		"guids": {dropletRequest.Data.GUID},
	}
	formatQueryParams(queryParameters)

	var matchedDroplets []*cfappsv1alpha1.Droplet
	matchedDroplets, err = getDropletListFromQuery(&a.Client, queryParameters)
	if err != nil {
		fmt.Printf("error fetching droplets from query: %s\n", err)
		errorMessage = "Error fetching droplet"
		errorTitle = "CF-ServerError"
		errorCode = 10001
		errorHeader = 500
		ReturnFormattedError(w, errorHeader, errorTitle, errorMessage, errorCode)
		return
	} else if len(matchedDroplets) == 0 {
		fmt.Println("Unable to assign current droplet. Ensure the droplet exists and belongs to this app.")
		errorMessage = "Unable to assign current droplet. Ensure the droplet exists and belongs to this app."
		errorTitle = "CF-UnprocessableEntity"
		errorCode = 10008
		errorHeader = 422
		ReturnFormattedError(w, errorHeader, errorTitle, errorMessage, errorCode)
		return
	}
	matchedDroplet := matchedDroplets[0]

	if matchedApp.ObjectMeta.Namespace != matchedDroplet.ObjectMeta.Namespace {
		fmt.Println("droplet doesn't exit in the same namespace as app")
		errorMessage = "Unable to assign current droplet. Ensure the droplet exists and belongs to this app."
		errorTitle = "CF-UnprocessableEntity"
		errorCode = 10008
		errorHeader = 422
		ReturnFormattedError(w, errorHeader, errorTitle, errorMessage, errorCode)
		return
	}

	if matchedApp.ObjectMeta.Name != matchedDroplet.Spec.AppRef.Name {
		fmt.Println("Unable to assign current droplet. Ensure the droplet exists and belongs to this app.")
		errorMessage = "Unable to assign current droplet. Ensure the droplet exists and belongs to this app."
		errorTitle = "CF-UnprocessableEntity"
		errorCode = 10008
		errorHeader = 422
		ReturnFormattedError(w, errorHeader, errorTitle, errorMessage, errorCode)
		return
	}

	matchedApp.Spec.CurrentDropletRef = cfappsv1alpha1.DropletReference{
		APIVersion: "apps.cloudfoundry.org/v1alpha1",
		Kind:       "Droplet",
		Name:       dropletRequest.Data.GUID,
	}

	err = a.Client.Update(context.Background(), matchedApp)
	if err != nil {
		fmt.Printf("error updating App object: %v\n", err)
		errorMessage = "Error updating app object"
		errorTitle = "CF-ServerError"
		errorCode = 10001
		errorHeader = 500
		ReturnFormattedError(w, errorHeader, errorTitle, errorMessage, errorCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(formatSetDropletResponse(matchedApp))
}

func (a *AppHandler) ReturnFormattedResponse(w http.ResponseWriter, app *cfappsv1alpha1.App) {
	formattedApp := formatAppToPresenter(app)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(formattedApp)
}

// SetAppDesiredStateHandler is for getting a single app from the guid
// For now, only outputs the first match after searching ALL namespaces for Apps
// POST /v3/apps/:guid/actions/start|stop
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#start-an-app
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#stop-an-app
func (a *AppHandler) SetAppDesiredStateHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch the {guid} value from URL using gorilla mux
	vars := mux.Vars(r)
	appGUID := vars["guid"]
	action := vars["action"]
	desiredState := cfappsv1alpha1.StartedState

	if action != "start" && action != "stop" {
		ReturnFormattedError(w, 500, "ServerError", fmt.Sprintf("action %s is not recognized", action), 10001)
		return
	} else if action == "stop" {
		desiredState = cfappsv1alpha1.StoppedState
	}

	// map[string][]string
	queryParameters := map[string][]string{
		"guids": {appGUID},
	}

	// Use the k8s client to fetch the apps with the same metadata.name as the guid
	matchedApps, err := getAppListFromQuery(&a.Client, queryParameters)
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	if len(matchedApps) < 1 {
		// If no matches for the GUID, just return a 404
		errorMessage := fmt.Sprintf("App with guid %s not found", appGUID)
		errorTitle := "CF-ResourceNotFound"
		errorCode := 10010
		errorHeader := 404
		ReturnFormattedError(w, errorHeader, errorTitle, errorMessage, errorCode)
		return
	}

	matchedApp := matchedApps[0]
	if matchedApp.Spec.DesiredState != desiredState {
		// modify the desired state of the app and re-apply it
		matchedApp.Spec.DesiredState = desiredState
		err = a.Client.Update(context.Background(), matchedApp)
		if err != nil {
			fmt.Printf("error updating App object: %v\n", err)
			w.WriteHeader(500)
			return
		}
	}

	// Write MatchedApps to http ResponseWriter
	w.Header().Set("Content-Type", "application/json")
	// We are only printing the first element in the list for now ignoring cross-namespace guid collisions
	a.ReturnFormattedResponse(w, matchedApp)
}
