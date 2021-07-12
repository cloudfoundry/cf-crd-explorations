package handlers

import (
	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	"cloudfoundry.org/cf-crd-explorations/cfshim/filters"
	"cloudfoundry.org/cf-crd-explorations/settings"
	"context"
	"encoding/json"
	"fmt"
	"github.com/buildpacks/pack/pkg/archive"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// Define the routes used in the REST endpoints
const (
	PackageEndpoint       = "/v3/packages"
	UploadPackageEndpoint = "/v3/packages/{guid}/upload"
	GetPackageEndpoint    = PackageEndpoint + "/{guid}"
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

func (p *PackageHandler) ReturnFormattedResponse(w http.ResponseWriter, packageGUID string, username string) {

	//reuse the getAppHelper method to fetch and return the app in the HTTP response.
	queryParameters := map[string][]string{
		"guids": {packageGUID},
	}

	// Convert to a list of CFAPIAppResource to match old Cloud Controller Formatting in REST response
	matchedPackages, err := p.getPackageHelper(queryParameters)
	if err != nil || len(matchedPackages) == 0 {
		fmt.Printf("error fecthing the created package: %s\n", err)
		w.WriteHeader(500)
		return
	}

	formattedPackage := formatPackageResponse(matchedPackages[0], username)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(formattedPackage)
}

func formatPackageResponse(pk *appsv1alpha1.Package, username string) CFAPIPackageResource {
	return CFAPIPackageResource{
		Type: string(pk.Spec.Type),
		Relationships: CFAPIPackageAppRelationships{
			App: CFAPIPackageAppRelationshipsApp{
				Data: CFAPIPackageAppRelationshipsAppData{
					GUID: pk.Spec.AppRef.Name,
				},
			},
		},
		Data: CFAPIPackageData{
			Image:    pk.Spec.Source.Registry.Image,
			Username: username,
			Password: "****",
		},
	}
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

func (p *PackageHandler) getAppHelper(queryParameters map[string][]string) ([]CFAPIAppResource, error) {
	// use a helper function to break comma separated values into []string
	formatQueryParams(queryParameters)

	// Apply filter to AllPackages and store result in matchedApps
	matchedApps, err := p.getAppListFromQuery(queryParameters)
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

func (p *PackageHandler) getAppListFromQuery(queryParameters map[string][]string) ([]*appsv1alpha1.App, error) {
	var filter Filter = &filters.AppFilter{
		QueryParameters: queryParameters,
	}

	// Get all the CF Apps from K8s API store in AllPackages which contains Items: []App
	AllPackages := &appsv1alpha1.AppList{}
	err := p.Client.List(context.Background(), AllPackages)
	if err != nil {
		return nil, fmt.Errorf("error fetching app: %v", err)
	}

	// Apply filter to AllPackages and store result in matchedApps
	var matchedApps []*appsv1alpha1.App
	fmt.Printf("%v\n", matchedApps)
	for i, _ := range AllPackages.Items {
		if filter.Filter(&AllPackages.Items[i]) {
			matchedApps = append(matchedApps, &AllPackages.Items[i])
		}
	}
	return matchedApps, nil
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
		fmt.Printf("error fecthing the package: %s\n", err)
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
	queryParams := map[string][]string{
		"guids": {packageRequest.Relationships.App.Data.GUID},
	}

	appResources, err := p.getAppHelper(queryParams)
	if err != nil {
		// Print the error if K8s client fails
		w.WriteHeader(500)
		fmt.Fprintf(w, "Failed to get the App namespace %v", err)
		return
	}

	if len(appResources) == 0 {
		w.WriteHeader(422)
		fmt.Fprintf(w, "Failed to create package as App does not exist")
		return
	}

	namespace := appResources[0].Relationships.Space.Data.GUID

	packageGUID := uuid.NewString()

	secretObj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatePackageSecretName(packageGUID),
			Namespace: namespace,
		},
		StringData: map[string]string{"username": packageRequest.Data.Username, "password": packageRequest.Data.Password},
	}
	err = p.Client.Create(context.Background(), secretObj)
	if err != nil {
		fmt.Printf("error creating Secret object: %v\n", *secretObj)
		w.WriteHeader(500)
	}

	pk := &appsv1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      packageGUID,
			Namespace: namespace, // how do we ensure that the namespace exists?
		},
		Spec: appsv1alpha1.PackageSpec{
			Type: appsv1alpha1.PackageType("docker"),
			AppRef: appsv1alpha1.ApplicationReference{
				Name: packageRequest.Relationships.App.Data.GUID,
			},
			Source: appsv1alpha1.PackageSource{
				Registry: appsv1alpha1.Registry{
					Image: packageRequest.Data.Image,
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: packageGUID + "-secret"},
					},
				},
			},
		},
	}

	err = p.Client.Create(context.Background(), pk)
	if err != nil {
		fmt.Printf("error creating Package object: %v\n", *pk)
		w.WriteHeader(500)
		return
	}

	p.ReturnFormattedResponse(w, packageGUID, packageRequest.Data.Username)
}

// dumb helper function to make the secret name for a docker package in case we want to change it later.
func generatePackageSecretName(packageGUID string) string {
	return packageGUID + "-secret"
}

// POST /v3/packages/:guid/upload
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#upload-package-bits
func (p *PackageHandler) UploadPackageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	packageGuid := vars["guid"]
	ctx := r.Context()

	packages, err := p.getPackagesListFromQuery(map[string][]string{
		"guids": {packageGuid},
	})
	if len(packages) == 0 {
		returnFormattedError(w, 404, "NotFound", "", 10000)
	}
	pkg := packages[0]

	packageBitsFile, _, err := r.FormFile("bits")
	if err != nil {
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}
	defer packageBitsFile.Close()

	tmpFile, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("package-%s-", packageGuid))
	if err != nil {
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}
	defer os.Remove(tmpFile.Name())

	fmt.Println("Created tmp file: " + tmpFile.Name())

	if _, err = io.Copy(tmpFile, packageBitsFile); err != nil {
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	// Close the packageBitsFile
	if err := tmpFile.Close(); err != nil {
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	image, err := random.Image(0, 0)
	if err != nil {
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	noopFilter := func(string) bool { return true }
	layer, err := tarball.LayerFromReader(archive.ReadZipAsTar(tmpFile.Name(), "/", 0, 0, -1, true, noopFilter))
	if err != nil {
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	image, err = mutate.AppendLayers(image, layer)
	if err != nil {
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	authenticator := authn.FromConfig(authn.AuthConfig{
		Username: settings.GlobalSettings.PackageRegistryUsername,
		Password: settings.GlobalSettings.PackageRegistryPassword,
	})
	registryBasePath := settings.GlobalSettings.PackageRegistryTagBase

	ref, err := name.ParseReference(fmt.Sprintf("%s/%s", registryBasePath, packageGuid))
	if err != nil {
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	err = remote.Write(ref, image, remote.WithAuth(authenticator))
	if err != nil {
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	updatedPkg := pkg.DeepCopy()
	// TODO: Update Package CR spec on K8s
	updatedPkg.Spec.Source = appsv1alpha1.PackageSource{
		Registry: appsv1alpha1.Registry{
			Image:            ref.Name(),
			ImagePullSecrets: nil, // TODO: What goes here? Maybe take this secret name as a config?
		},
	}
	meta.SetStatusCondition(&updatedPkg.Status.Conditions, metav1.Condition{
		Type:               "Succeeded",
		Status:             metav1.ConditionTrue,
		Reason:             "Uploaded",
		Message:            "",
	})
	meta.SetStatusCondition(&updatedPkg.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Uploaded",
		Message:            "",
	})
	meta.SetStatusCondition(&updatedPkg.Status.Conditions, metav1.Condition{
		Type:               "Uploaded",
		Status:             metav1.ConditionTrue,
		Reason:             "Uploaded",
		Message:            "",
	})
	err = p.Client.Patch(ctx, updatedPkg, client.MergeFrom(pkg))
	if err != nil {
		returnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	// Probably punting on this until the GET /v3/packages/:guid endpoint is implemented
	// NOTE : based on knowledge shared by Birdrock about CREATE writing to etcd and GET/LIST reading from cache, which\
	//		  can result in cache miss should it be sufficient to return the updated object itself rather than\
	// 		  invoking the GET/LIST?

	formattedMatchingPackage := formatPresenterPackageResponse(updatedPkg)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(formattedMatchingPackage)
}

// getPackageListFromQuery takes URL query parameters and queries the K8s Client for all Packages
// builds a filter based on params and walks through, placing every match into the returned list of Packages
// returns an error if something went wrong with the K8s query
func (p *PackageHandler) getPackagesListFromQuery(queryParameters map[string][]string) ([]*appsv1alpha1.Package, error) {
	var filter Filter = &filters.PackageFilter{
		QueryParameters: queryParameters,
	}

	// Get all the CF Apps from K8s API store in AllPackages which contains Items: []App
	AllPackages := &appsv1alpha1.PackageList{}
	err := p.Client.List(context.Background(), AllPackages)
	if err != nil {
		return nil, fmt.Errorf("error fetching app: %v", err)
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
