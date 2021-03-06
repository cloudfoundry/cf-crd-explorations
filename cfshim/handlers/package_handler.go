package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	"cloudfoundry.org/cf-crd-explorations/cfshim/filters"
	"cloudfoundry.org/cf-crd-explorations/settings"
	"github.com/buildpacks/pack/pkg/archive"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/pivotal/kpack/pkg/registry"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Define the routes used in the REST endpoints
const (
	PackageEndpoint       = "/v3/packages"
	UploadPackageEndpoint = "/v3/packages/{guid}/upload"
	GetPackageEndpoint    = PackageEndpoint + "/{guid}"
)

type PackageHandler struct {
	// This is a Kuberentes client, contains authentication and context stuff for running K8s queries
	Client          client.Client
	KeychainFactory registry.KeychainFactory
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

	app := matchedApps[0]
	namespace := app.Namespace
	packageGUID := uuid.NewString()

	pk := &appsv1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      packageGUID,
			Namespace: namespace,
			Labels:    map[string]string{LabelAppGUID: app.Name},
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

// POST /v3/packages/:guid/upload
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#upload-package-bits
func (p *PackageHandler) UploadPackageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	packageGuid := vars["guid"]
	ctx := r.Context()

	packages, err := getPackagesListFromQuery(&p.Client, map[string][]string{
		"guids": {packageGuid},
	})
	if len(packages) == 0 {
		ReturnFormattedError(w, 404, "NotFound", "", 10000)
	}
	pkg := packages[0]

	packageBitsFile, _, err := r.FormFile("bits")
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}
	defer packageBitsFile.Close()

	tmpFile, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("package-%s", packageGuid))
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}
	defer os.Remove(tmpFile.Name())

	fmt.Println("Created tmp file: " + tmpFile.Name())

	if _, err = io.Copy(tmpFile, packageBitsFile); err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	// Close the packageBitsFile
	if err := tmpFile.Close(); err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	image, err := random.Image(0, 0)
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	noopFilter := func(string) bool { return true }
	layer, err := tarball.LayerFromReader(archive.ReadZipAsTar(tmpFile.Name(), "/", 0, 0, -1, true, noopFilter))
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	image, err = mutate.AppendLayers(image, layer)
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	registrySecretName := settings.GlobalSettings.RegistrySecret
	packageRegistryBasePath := settings.GlobalSettings.PackageRegistryBase

	ref, err := name.ParseReference(fmt.Sprintf("%s/%s", packageRegistryBasePath, packageGuid))
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	keychain, err := p.KeychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{
		Namespace:        "default",
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: registrySecretName}},
	})
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	err = remote.Write(ref, image, remote.WithAuthFromKeychain(keychain))
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	updatedPkg := pkg.DeepCopy()

	imgDigest, err := image.Digest()
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	updatedPkg.Status.Checksum = appsv1alpha1.Checksum{
		Type:  "sha256",
		Value: imgDigest.String(),
	}

	updatedPkg.Spec.Source = appsv1alpha1.PackageSource{
		Registry: appsv1alpha1.Registry{
			Image:            ref.Name(),
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: registrySecretName}},
		},
	}
	meta.SetStatusCondition(&updatedPkg.Status.Conditions, metav1.Condition{
		Type:    "Succeeded",
		Status:  metav1.ConditionTrue,
		Reason:  "Uploaded",
		Message: "",
	})
	meta.SetStatusCondition(&updatedPkg.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "Uploaded",
		Message: "",
	})
	meta.SetStatusCondition(&updatedPkg.Status.Conditions, metav1.Condition{
		Type:    "Uploaded",
		Status:  metav1.ConditionTrue,
		Reason:  "Uploaded",
		Message: "",
	})
	err = p.Client.Patch(ctx, updatedPkg, client.MergeFrom(pkg))
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	err = p.Client.Status().Update(ctx, updatedPkg)
	if err != nil {
		ReturnFormattedError(w, 500, "ServerError", err.Error(), 10001)
		return
	}

	formattedMatchingPackage := formatPresenterPackageResponse(updatedPkg)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(formattedMatchingPackage)
}
