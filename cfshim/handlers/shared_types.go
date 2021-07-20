package handlers

type Filter interface {
	// Filter takes an object, casts it uses preset filters and returns yes/no
	Filter(interface{}) bool
}

type CFAPILifecycle struct {
	Type string                  `json:"type"`
	Data CFAPIBuildLifecycleData `json:"data"`
}

type CFAPIAppLifecycle struct {
	Type string                `json:"type"`
	Data CFAPIAppLifecycleData `json:"data"`
}

type CFAPIAppRelationshipsDroplet struct {
	Data CFAPIAppRelationshipsDropletData `json:"data"`
}

type CFAPIAppRelationshipsDropletData struct {
	GUID string `json:"guid"`
}

type CFAPILink struct {
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
