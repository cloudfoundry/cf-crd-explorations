package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	"cloudfoundry.org/cf-crd-explorations/cfshim/filters"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Define the routes used in the REST endpoints
const (
	PackageEndpoint    = "/v3/packages"
	GetPackageEndpoint = PackageEndpoint + "/{guid}"
)

type PackageHandler struct {
	// This is a Kuberentes client, contains authentication and context stuff for running K8s queries
	Client client.Client
}

func (p *PackageHandler) getPackageHelper(queryParameters map[string][]string) ([]*appsv1alpha1.Package, error) {
	var filter Filter = &filters.PackageFilter{
		QueryParameters: queryParameters,
	}

	AllPackages := &appsv1alpha1.PackageList{}
	err := p.Client.List(context.Background(), AllPackages)
	if err != nil {
		return nil, fmt.Errorf("error fetching package: %v", err)
	}

	var matchedPackages []*appsv1alpha1.Package
	//fmt.Printf("%v\n", matchedPackages)
	for i, _ := range AllPackages.Items {
		if filter.Filter(&AllPackages.Items[i]) {
			matchedPackages = append(matchedPackages, &AllPackages.Items[i])
		}
	}

	return matchedPackages, nil
}

func (p *PackageHandler) ReturnFormattedResponse(w http.ResponseWriter, formattedPackage *CFAPIPresenterPackageResource) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(*formattedPackage)
}

// formatPresenterPackageResponse Given a CR package, convert to the CF API response format:
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#create-a-package
func formatPresenterPackageResponse(pk *appsv1alpha1.Package) CFAPIPresenterPackageResource {
	toReturn := CFAPIPresenterPackageResource{
		GUID: pk.Name,
		Type: string(pk.Spec.Type),
		Data: CFAPIPresenterPackageData{
			// This Data.Type field is hidden from the Marshalled JSON
			//	it is used to format the JSON for bits and docker types differently
			Type: string(pk.Spec.Type),
		},
		// TODO: State is missing from package spec!!!
		State:     "",
		CreatedAt: pk.CreationTimestamp.UTC().Format(time.RFC3339),
		// TODO: Not sure how to get updated time, it is not present on CR for free
		UpdatedAt: "",
		Relationships: CFAPIPackageAppRelationships{
			App: CFAPIPackageAppRelationshipsApp{
				Data: CFAPIPackageAppRelationshipsAppData{
					GUID: pk.Spec.AppRef.Name,
				},
			},
		},
		// URL information about the server where you sub in the package GUID..
		Links: map[string]CFAPIAppLink{},
		Metadata: CFAPIMetadata{
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	if toReturn.Type == "bits" {
		toReturn.Data.Checksum = &CFAPIPresenterChecksum{
			Type:  "sha256",
			Value: nil,
		}
		toReturn.Data.Error = nil
	} else if toReturn.Type == "docker" {
		toReturn.Data.Image = pk.Spec.Source.Registry.Image
	}

	return toReturn
}

// GetPackageHandler is for getting a single package from the guid
// For now, only outputs the first match after searching ALL namespaces for Packages
// GET /v3/packages/:guid
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#get-a-package
func (p *PackageHandler) GetPackageHandler(w http.ResponseWriter, r *http.Request) {
	//Fetch the {guid} value from URL using gorilla mux
	vars := mux.Vars(r)
	packageGUID := vars["guid"]
	//reuse the getAppHelper method to fetch and return the app in the HTTP response.
	queryParameters := map[string][]string{
		"guids": {packageGUID},
	}
	// Convert to a list of CFAPIAppResource to match old Cloud Controller Formatting in REST response
	matchedPackages, err := p.getPackageHelper(queryParameters)
	if err != nil {
		fmt.Printf("error fetching the package: %s\n", err)
		w.WriteHeader(500)
		return
	} else if len(matchedPackages) < 1 {
		// If no matches for the GUID(metadata.name), just return a 404
		w.WriteHeader(404)
		return
	}

	// Convert the first matched package to the PresenterPackage JSON format
	firstMatchedPackage := matchedPackages[0]
	formattedMatchingPackage := formatPresenterPackageResponse(firstMatchedPackage)

	// for Docker packages we need to look up if it has a secret for username & password?
	// TODO: Standardize on the secrets approach - Staging flow uses image pull secrets which are different from the Package create API right now
	if formattedMatchingPackage.Type == "docker" {
		packageSecret, err := p.getSecretHelper(firstMatchedPackage.Namespace, generatePackageSecretName(firstMatchedPackage.Name))
		//fmt.Printf("Package secret was: %+v \nError: %v", packageSecret, err)
		if err == nil {
			// get the username field and decode it
			if usernameEncoded, exists := packageSecret.Data["username"]; exists {
				formattedMatchingPackage.Data.Username = string(usernameEncoded)
			}

			// get the password field and see if it exists
			if _, exists := packageSecret.Data["password"]; exists {
				formattedMatchingPackage.Data.Password = "***"
			}
		}
	}

	// Output the Package to the http response writer
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(formattedMatchingPackage)
}

// getSecretHelper returns a secret given its namespace and name. Returns nil and an error if not found.
func (p *PackageHandler) getSecretHelper(namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := p.Client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		} else {
			return nil, fmt.Errorf("error fetching secret object %s: %v", name, err)
		}
	}
	return secret, err
}

func (p *PackageHandler) CreatePackageHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var packageRequest CFAPIPackageResource
	err := json.NewDecoder(r.Body).Decode(&packageRequest)
	if err != nil {
		fmt.Printf("error parsing request: %s\n", err)
		w.WriteHeader(400)
	}

	// Get the app from the packageRequest relationships
	queryParams := map[string][]string{
		"guids": {packageRequest.Relationships.App.Data.GUID},
	}
	matchedApps, err := getAppListFromQuery(&p.Client, queryParams)
	if err != nil {
		// Print the error if K8s client fails
		w.WriteHeader(500)
		fmt.Fprintf(w, "Failed to get the App namespace %v", err)
		return
	}
	if len(matchedApps) == 0 {
		w.WriteHeader(422)
		fmt.Fprintf(w, "Failed to create package as App does not exist")
		return
	}

	namespace := matchedApps[0].Namespace

	packageGUID := uuid.NewString()
	pk := &appsv1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      packageGUID,
			Namespace: namespace, // how do we ensure that the namespace exists?
		},
		Spec: appsv1alpha1.PackageSpec{
			Type: appsv1alpha1.PackageType(packageRequest.Type),
			AppRef: appsv1alpha1.ApplicationReference{
				Name: packageRequest.Relationships.App.Data.GUID,
			},
		},
	}

	// Validate that the Docker image field has been provided
	if packageRequest.Type == "docker" {
		if packageRequest.Data == nil || packageRequest.Data.Image == "" {
			errorMessage := "Data Image must be a string, Data Image required"
			ReturnFormattedError(w, 422, "CF-UnprocessableEntity", errorMessage, 10008)
			return
		}

		// Add the docker image to the package spec
		pk.Spec.Source = appsv1alpha1.PackageSource{
			Registry: appsv1alpha1.Registry{
				Image: packageRequest.Data.Image,
			},
		}

		// If username and password are provided for the image, create a secret containing them
		if packageRequest.Data.Username != nil && packageRequest.Data.Password != nil {
			secretName := generatePackageSecretName(packageGUID)

			// Only create a secret if type is Docker and we have a username + password in packageRequest
			secretObj := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      generatePackageSecretName(packageGUID),
					Namespace: namespace,
				},
				StringData: map[string]string{"username": *packageRequest.Data.Username, "password": *packageRequest.Data.Password},
			}
			err = p.Client.Create(context.Background(), secretObj)
			if err != nil {
				fmt.Printf("error creating docker package Secret object: %v\n", err)
				w.WriteHeader(500)
			}
			// Add the secret details to our desired Package that we will create
			pk.Spec.Source.Registry.ImagePullSecrets = []corev1.LocalObjectReference{
				{Name: secretName},
			}
		}
	}

	err = p.Client.Create(context.Background(), pk)
	if err != nil {
		fmt.Printf("error creating Package object: %v\n", *pk)
		w.WriteHeader(500)
		return
	}

	// Format the in-memory package to the PresenterPackage type to match the CF API output JSON
	formattedPackage := formatPresenterPackageResponse(pk)
	if packageRequest.Type == "docker" && packageRequest.Data.Username != nil {
		updateDockerPackageResponse(&formattedPackage, *packageRequest.Data.Username)
	}

	p.ReturnFormattedResponse(w, &formattedPackage)
}

// updateDockerPackageResponse updates formattedPackage in-place to add Docker username and password fields
func updateDockerPackageResponse(formattedPackage *CFAPIPresenterPackageResource, username string) *CFAPIPresenterPackageResource {
	formattedPackage.Data.Username = username
	formattedPackage.Data.Password = "***"
	return formattedPackage
}

// dumb helper function to make the secret name for a docker package in case we want to change it later.
func generatePackageSecretName(packageGUID string) string {
	return packageGUID + "-secret"
}
