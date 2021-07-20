package handlers

type CFAPIAppResource struct {
	GUID          string                `json:"guid"`
	Name          string                `json:"name"`
	State         string                `json:"state"`
	CreatedAt     string                `json:"created_at"`
	UpdatedAt     string                `json:"updated_at"`
	Lifecycle     CFAPILifecycle        `json:"lifecycle,omitempty"`
	Relationships CFAPIAppRelationships `json:"relationships"`
	Links         map[string]CFAPILink  `json:"links"`
	Metadata      CFAPIMetadata         `json:"metadata"`
}

type CFAPIAppResourceWithEnvVars struct {
	CFAPIAppResource
	EnvironmentVariables map[string]string `json:"environment_variables,omitempty"`
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
