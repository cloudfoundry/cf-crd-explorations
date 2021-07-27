package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"encoding/json"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Define the routes used in the REST endpoints
const (
	BuildsEndpoint    = "/v3/builds"
	GetBuildsEndpoint = BuildsEndpoint + "/{guid}"
)

type BuildHandler struct {
	// This is a Kuberentes client, contains authentication and context stuff for running K8s queries
	Client client.Client
}

// GetBuildHandler is for getting a single build from the guid
// For now, only outputs the first match after searching ALL namespaces for Builds
// GET /v3/builds/:guid
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#get-a-build
func (b *BuildHandler) GetBuildHandler(w http.ResponseWriter, r *http.Request) {
	//Fetch the {guid} value from URL using gorilla mux
	vars := mux.Vars(r)
	buildGUID := vars["guid"]

	// map[string][]string
	queryParameters := map[string][]string{
		"guids": {buildGUID},
	}

	// Use the k8s client to fetch the builds with the same metadata.name as the guid
	matchedBuilds, err := getBuildListFromQuery(&b.Client, queryParameters)
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	if len(matchedBuilds) < 1 {
		// If no matches for the GUID, just return a 404
		w.WriteHeader(404)
		return
	}

	// Write MatchedBuilds to http ResponseWriter
	w.Header().Set("Content-Type", "application/json")
	// We are only printing the first element in the list for now ignoring cross-namespace guid collisions
	b.ReturnFormattedResponse(w, matchedBuilds[0])
}

func (b *BuildHandler) CreateBuildsHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := context.Background()

	var buildRequest CFAPIBuildResource
	var errStrings []string

	decoder := json.NewDecoder(r.Body)
	// body, _ := ioutil.ReadAll((r.Body))
	// fmt.Printf("Body: %v", string(body))
	decoder.DisallowUnknownFields()

	err := decoder.Decode(&buildRequest)
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

	if buildRequest.Package == nil {
		buf := new(strings.Builder)
		p, err := io.Copy(buf, r.Body)
		if err != nil {
			errStrings = append(errStrings, err.Error())
		}
		errStrings = append(errStrings, fmt.Sprintf("The request is semantically invalid: %v", p))
	}

	if len(errStrings) > 0 {
		errorDetail := strings.Join(errStrings, ", ")
		ReturnFormattedError(w, 422, "CF-UnprocessableEntity", errorDetail, 10008)
		return
	}

	lifecycleType := buildRequest.Lifecycle.Type
	if lifecycleType == "" {
		lifecycleType = string(appsv1alpha1.BuildpackLifecycle)
	}

	lifecycleData := buildRequest.Lifecycle.Data
	if lifecycleType == string(appsv1alpha1.BuildpackLifecycle) && lifecycleData.Stack == "" {
		lifecycleData.Stack = "cflinuxfs3" // TODO: This is the default in CF for VMs. What should the default stack be here?
	}
	if lifecycleType == string(appsv1alpha1.BuildpackLifecycle) && len(lifecycleData.Buildpacks) == 0 {
		lifecycleData.Buildpacks = []string{}
	}

	// Check if the package in the request exists
	buildPackages := &appsv1alpha1.PackageList{}

	err = b.Client.List(ctx, buildPackages, client.MatchingFields{"metadata.name": buildRequest.Package.GUID})
	// err = b.Client.Get(ctx, types.NamespacedName{Name: buildRequest.Package.GUID}, buildPackage)
	// TODO: Check for duplicate GUIDs (oh no) in different namespaces
	if err != nil {
		if apierrors.IsNotFound(err) {
			ReturnFormattedError(w, 404, "NotFound", err.Error(), 10000)
		} else {
			fmt.Printf("error fetching Namespace object: %v\n", *buildPackages)
			ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		}
		return
	}

	if len(buildPackages.Items) > 1 {
		ReturnFormattedError(w, 500, "ServerError", errors.New("package found in multiple namespaces").Error(), 10001)
		return
	}

	if len(buildPackages.Items) == 0 {
		ReturnFormattedError(w, 500, "ServerError", errors.New("no packages found").Error(), 10001)
		return
	}

	buildPackage := buildPackages.Items[0]

	//generate new UUID for each create build request.
	buildGUID := uuid.NewString()

	if buildRequest.Metadata.Labels == nil {
		buildRequest.Metadata.Labels = make(map[string]string)
	}
	buildRequest.Metadata.Labels[LabelAppGUID] = buildPackage.Labels[LabelAppGUID]
	buildRequest.Metadata.Labels[LabelPackageGUID] = buildPackage.Name

	build := &appsv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:        buildGUID,
			Namespace:   buildPackage.Namespace,
			Labels:      buildRequest.Metadata.Labels,
			Annotations: buildRequest.Metadata.Annotations,
		},
		Spec: appsv1alpha1.BuildSpec{
			Type: appsv1alpha1.LifecycleType(lifecycleType),
			LifecycleData: appsv1alpha1.LifecycleData{
				Buildpacks: lifecycleData.Buildpacks,
				Stack:      lifecycleData.Stack,
			},
			KpackBuildSelector: appsv1alpha1.KpackBuildSelector{
				MatchLabels: map[string]string{},
			},
			PackageRef: appsv1alpha1.PackageReference{
				Kind:       "Package",
				APIVersion: "apps.cloudfoundry.org/v1alpha1",
				Name:       buildRequest.Package.GUID,
			},
			AppRef: appsv1alpha1.ApplicationReference{
				Kind:       "App",
				APIVersion: "apps.cloudfoundry.org/v1alpha1",
				Name:       buildPackage.Spec.AppRef.Name,
			},
		},
	}

	err = b.Client.Create(ctx, build)
	if err != nil {
		fmt.Printf("error creating Build object: %v\n", err)
		w.WriteHeader(500)
		return
	}

	b.ReturnFormattedResponse(w, build)
}

func (b *BuildHandler) ReturnFormattedResponse(w http.ResponseWriter, build *appsv1alpha1.Build) {
	formattedBuild := formatBuildToPresenter(build)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(formattedBuild)
}
