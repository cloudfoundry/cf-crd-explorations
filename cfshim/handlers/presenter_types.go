package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
)

// Presenters- used for cfshim api responses
// 		we have separated the structs becuase some fields need to be hidden/shown differently to match the CF api
//---------------------------------------------------------------------------------------
// APP PRESENTER
//---------------------------------------------------------------------------------------
// Used to present App data in cf api output format.
// Must be a separate struct to include custom JSON Marshal function to hide/show certain fields to match CF API.
type CFAPIPresenterAppResource struct {
	GUID          string                     `json:"guid"`
	Name          string                     `json:"name"`
	State         string                     `json:"state"`
	CreatedAt     string                     `json:"created_at"`
	UpdatedAt     string                     `json:"updated_at"`
	Lifecycle     CFAPIPresenterAppLifecycle `json:"lifecycle,omitempty"`
	Relationships CFAPIAppRelationships      `json:"relationships"`
	Links         map[string]CFAPILink       `json:"links"`
	Metadata      CFAPIMetadata              `json:"metadata"`
}

type CFAPIPresenterBuildResource struct {
	GUID          string                     `json:"guid"`
	State         string                     `json:"state"`
	CreatedAt     string                     `json:"created_at"`
	UpdatedAt     string                     `json:"updated_at"`
	Lifecycle     CFAPIPresenterAppLifecycle `json:"lifecycle,omitempty"`
	Relationships CFAPIAppRelationships      `json:"relationships"`
	Links         map[string]CFAPILink       `json:"links"`
	Metadata      CFAPIMetadata              `json:"metadata"`
}

type CFAPIPresenterAppLifecycle struct {
	Type string                         `json:"type"`
	Data CFAPIPresenterAppLifecycleData `json:"data"`
}

// This looks dumb, but it makes the MarshalJSON function below not be infinitely recursive
type CFAPIPresenterBuildpackAppLifecycle struct {
	Type string                         `json:"type"`
	Data CFAPIPresenterAppLifecycleData `json:"data"`
}

type CFAPIPresenterAppLifecycleData struct {
	Buildpacks []string `json:"buildpacks"`
	Stack      string   `json:"stack"`
}

// Need this struct to marshal data to always be {}, empty JSON- otherwise CFAPIPresenterAppLifecycleData would show up
type CFAPIPresenterAppDockerLifecycle struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// We need a custom MarshalJSON function to implement the encoding/json.Marshaler interface
// This will let us present the data in CFAPIPresenterAppLifecycle two different ways depending on the Type field
// https://attilaolah.eu/2013/11/29/json-decoding-in-go/
func (l CFAPIPresenterAppLifecycle) MarshalJSON() ([]byte, error) {
	if l.Type == "buildpack" {
		return json.Marshal(CFAPIPresenterBuildpackAppLifecycle(l))
	}

	return json.Marshal(CFAPIPresenterAppDockerLifecycle{
		Type: l.Type,
		// Force the data to look like {} instead of null
		Data: make(map[string]interface{}),
	})
}

func formatAppToPresenter(app *appsv1alpha1.App) CFAPIPresenterAppResource {
	toReturn := CFAPIPresenterAppResource{
		GUID:      app.Name,
		Name:      app.Spec.Name,
		State:     string(app.Spec.DesiredState),
		CreatedAt: app.CreationTimestamp.UTC().Format(time.RFC3339),
		UpdatedAt: "",
		Lifecycle: CFAPIPresenterAppLifecycle{
			Type: string(app.Spec.Type),
			Data: CFAPIPresenterAppLifecycleData{
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
		Links: map[string]CFAPILink{},
		Metadata: CFAPIMetadata{
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	updatedAt, err := getTimeLastUpdatedTimestamp(&app.ObjectMeta)
	if err != nil {
		fmt.Printf("Error finding last updated time for app %s: %v\n", app.Name, err)
	}
	toReturn.UpdatedAt = updatedAt
	return toReturn
}

func formatBuildToPresenter(build *appsv1alpha1.Build) CFAPIBuildResource {
	var dropletRef *CFAPIBuildDroplet
	if build.Status.BuildDropletStatus != nil {
		dropletRef = &CFAPIBuildDroplet{
			GUID: build.Name,
		}
	}

	toReturn := CFAPIBuildResource{
		GUID:      build.Name,
		State:     deriveBuildState(build.Status.Conditions),
		CreatedAt: build.CreationTimestamp.UTC().Format(time.RFC3339),
		UpdatedAt: "",
		Lifecycle: CFAPILifecycle{
			Type: string(build.Spec.Type),
			Data: CFAPIBuildLifecycleData{
				Buildpacks: build.Spec.LifecycleData.Buildpacks,
				Stack:      build.Spec.LifecycleData.Stack,
			},
		},
		Package: &CFAPIBuildPackage{
			GUID: build.Spec.PackageRef.Name, // TODO: Should we reject a story?
		},
		Droplet: dropletRef,
		Relationships: CFAPIBuildRelationships{
			App: CFAPIBuildRelationshipsApps{
				Data: CFAPIBuildRelationshipsAppsData{
					GUID: build.Spec.AppRef.Name,
				},
			},
		},
		// URL information about the server where you sub in the app GUID..
		Links: map[string]CFAPILink{},
		Metadata: CFAPIMetadata{
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	updatedAt, err := getTimeLastUpdatedTimestamp(&build.ObjectMeta)
	if err != nil {
		fmt.Printf("Error finding last updated time for build %s: %v\n", build.Name, err)
	}
	toReturn.UpdatedAt = updatedAt
	return toReturn
}

func deriveBuildState(conditions []metav1.Condition) string {
	if meta.IsStatusConditionTrue(conditions, appsv1alpha1.StagingConditionType) {
		return "STAGING"
	} else if meta.IsStatusConditionTrue(conditions, appsv1alpha1.SucceededConditionType) {
		return "STAGED"
	} else if meta.IsStatusConditionFalse(conditions, appsv1alpha1.SucceededConditionType) {
		return "FAILED"
	} else {
		// If we're in an Unknown state, then assume the CRD was just created and consider it staging
		return "STAGING"
	}
}

//---------------------------------------------------------------------------------------
// PACKAGE PRESENTER
//---------------------------------------------------------------------------------------
// Used to present Package data in cf api output format.
// Must be a separate struct to include custom JSON Marshal function to hide/show certain fields to match CF API.
type CFAPIPresenterPackageResource struct {
	GUID          string                       `json:"guid"`
	Type          string                       `json:"type"`
	Data          CFAPIPresenterPackageData    `json:"data"`
	State         string                       `json:"state"`
	CreatedAt     string                       `json:"created_at"`
	UpdatedAt     string                       `json:"updated_at"`
	Relationships CFAPIPackageAppRelationships `json:"relationships"`
	Links         map[string]CFAPILink         `json:"links"`
	Metadata      CFAPIMetadata                `json:"metadata"`
}

type CFAPIPresenterPackageData struct {
	CFAPIPresenterPackageDockerData
	CFAPIPresenterPackageBitsData
	Type string `json:"-"`
}
type CFAPIPresenterPackageBitsData struct {
	Checksum *CFAPIPresenterChecksum `json:"checksum,omitempty"`
	Error    *string                 `json:"error"`
}
type CFAPIPresenterPackageDockerData struct {
	Image    string `json:"image"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// We need a custom MarshalJSON function to implement the encoding/json.Marshaler interface
// This will let us present the data in CFAPIPresenterPackageData two different ways depending on the Type field
// https://attilaolah.eu/2013/11/29/json-decoding-in-go/
func (d CFAPIPresenterPackageData) MarshalJSON() ([]byte, error) {
	if d.Type == "bits" {
		return json.Marshal(CFAPIPresenterPackageBitsData{
			Checksum: d.Checksum,
			Error:    d.Error,
		})
	} else if d.Type == "docker" {
		return json.Marshal(CFAPIPresenterPackageDockerData{
			Image:    d.Image,
			Username: d.Username,
			Password: d.Password,
		})
	}
	return json.Marshal(map[string]interface{}{})
}

type CFAPIPresenterChecksum struct {
	Type  string  `json:"type"`
	Value *string `json:"value"`
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
		State:     derivePackageState(pk.Status.Conditions),
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
		Links: map[string]CFAPILink{},
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
		toReturn.State = "READY"
	}

	updatedAt, err := getTimeLastUpdatedTimestamp(&pk.ObjectMeta)
	if err != nil {
		fmt.Printf("Error finding last updated time for package %s: %v\n", pk.Name, err)
	}
	toReturn.UpdatedAt = updatedAt
	return toReturn
}

func derivePackageState(Conditions []metav1.Condition) string {
	if meta.IsStatusConditionTrue(Conditions, "Succeeded") &&
		meta.IsStatusConditionTrue(Conditions, "Uploaded") &&
		meta.IsStatusConditionTrue(Conditions, "Ready") {
		return "READY"
	} else {
		return "AWAITING_UPLOAD"
	}
}

//---------------------------------------------------------------------------------------
// DROPLET PRESENTER
//---------------------------------------------------------------------------------------
type CFAPIPresenterAppRelationshipsDroplet struct {
	Data  CFAPIAppRelationshipsDropletData `json:"data"`
	Links map[string]CFAPILink             `json:"links"`
}

func formatSetDropletResponse(app *appsv1alpha1.App) CFAPIPresenterAppRelationshipsDroplet {
	return CFAPIPresenterAppRelationshipsDroplet{
		Data: CFAPIAppRelationshipsDropletData{
			GUID: app.Spec.CurrentDropletRef.Name,
		},
		Links: map[string]CFAPILink{},
	}
}
