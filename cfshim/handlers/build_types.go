package handlers

type CFAPIBuildResource struct {
	GUID          string                  `json:"guid"`
	State         string                  `json:"state"`
	CreatedAt     string                  `json:"created_at"`
	UpdatedAt     string                  `json:"updated_at"`
	Lifecycle     CFAPILifecycle          `json:"lifecycle,omitempty"`
	Package       *CFAPIBuildPackage      `json:"package"`
	Droplet       CFAPIBuildDroplet       `json:"droplet"`
	Relationships CFAPIBuildRelationships `json:"relationships"`
	Links         map[string]CFAPILink    `json:"links"`
	Metadata      CFAPIMetadata           `json:"metadata"`
}

type CFAPIBuildRelationships struct {
	App CFAPIBuildRelationshipsApps `json:"app"`
}

type CFAPIBuildRelationshipsApps struct {
	Data CFAPIBuildRelationshipsAppsData `json:"data"`
}

type CFAPIBuildRelationshipsAppsData struct {
	GUID string `json:"guid"`
}

type CFAPIBuildLifecycleData struct {
	Buildpacks []string `json:"buildpacks,omitempty"`
	Stack      string   `json:"stack,omitempty"`
}

type CFAPIBuildPackage struct {
	GUID string `json:"guid"`
}

type CFAPIBuildDroplet struct {
	GUID string `json:"guid"`
}
