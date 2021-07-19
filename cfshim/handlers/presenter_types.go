package handlers

import (
	"encoding/json"
	"time"

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
	Links         map[string]CFAPIAppLink    `json:"links"`
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
	return CFAPIPresenterAppResource{
		GUID:      app.Name,
		Name:      app.Spec.Name,
		State:     string(app.Spec.State),
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
		Links: map[string]CFAPIAppLink{},
		Metadata: CFAPIMetadata{
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
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
	Links         map[string]CFAPIAppLink      `json:"links"`
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

type CFAPIPresenterAppRelationshipsDroplet struct {
	Data  CFAPIAppRelationshipsDropletData `json:"data"`
	Links map[string]CFAPIAppLink          `json:"links,omitempty"`
}

func formatSetDropletResponse(app *appsv1alpha1.App) CFAPIPresenterAppRelationshipsDroplet {
	return CFAPIPresenterAppRelationshipsDroplet{
		Data: CFAPIAppRelationshipsDropletData{
			GUID: app.Spec.CurrentDropletRef.Name,
		},
	}
}
