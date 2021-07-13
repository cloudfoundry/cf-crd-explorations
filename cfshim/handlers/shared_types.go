package handlers

import "encoding/json"

type CFAPIAppResource struct {
	GUID          string                  `json:"guid"`
	Name          string                  `json:"name"`
	State         string                  `json:"state"`
	CreatedAt     string                  `json:"created_at"`
	UpdatedAt     string                  `json:"updated_at"`
	Lifecycle     CFAPIAppLifecycle       `json:"lifecycle,omitempty"`
	Relationships CFAPIAppRelationships   `json:"relationships"`
	Links         map[string]CFAPIAppLink `json:"links"`
	Metadata      CFAPIMetadata           `json:"metadata"`
}

type CFAPIAppResourceWithEnvVars struct {
	CFAPIAppResource
	EnvironmentVariables map[string]string `json:"environment_variables,omitempty"`
}

type CFAPIAppLifecycle struct {
	Type string                `json:"type"`
	Data CFAPIAppLifecycleData `json:"data"`
}

type CFAPIAppLifecycleData struct {
	Buildpacks []string `json:"buildpacks,omitempty"`
	Stack      string   `json:"stack,omitempty"`
}

type CFAPIAppRelationships struct {
	Space CFAPIAppRelationshipsSpace `json:"space"`
}

type CFAPIAppRelationshipsSpace struct {
	Data CFAPIAppRelationshipsSpaceData `json:"data"`
}

type CFAPIAppRelationshipsSpaceData struct {
	GUID string `json:"guid"`
}

type CFAPIAppLink struct {
	Href   string `json:"href"`
	Method string `json:"method,omitempty"`
}

type CFAPIMetadata struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type CFAPIPackageResource struct {
	Type          string                       `json:"type"`
	Relationships CFAPIPackageAppRelationships `json:"relationships"`
	Data          *CFAPIPackageData            `json:"data,omitempty"`
}

type CFAPIPackageAppRelationships struct {
	App CFAPIPackageAppRelationshipsApp `json:"app"`
}

type CFAPIPackageAppRelationshipsApp struct {
	Data CFAPIPackageAppRelationshipsAppData `json:"data"`
}

type CFAPIPackageAppRelationshipsAppData struct {
	GUID string `json:"guid"`
}

type CFAPIPackageData struct {
	Image    string  `json:"image"`
	Username *string `json:"username,omitempty"`
	Password *string `json:"password,omitempty"`
}

type CFAPIErrors struct {
	Errors []CFAPIError `json:"errors"`
}

type CFAPIError struct {
	Detail string `json:"detail"`
	Title  string `json:"title"`
	Code   int    `json:"code"`
}

// Presenters- used for cfshim api responses
// TODO: Pull out presenters into their own file/package if we add more
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
