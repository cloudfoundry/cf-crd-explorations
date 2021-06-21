package handlers

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
