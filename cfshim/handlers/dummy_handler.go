package handlers

import (
	"net/http"
)

// Define the routes used in the REST endpoints
const (
	RootEndpoint   = "/"
	OrgsEndpoint   = "/v3/organizations"
	SpacesEndpoint = "/v3/spaces"
)

type DummyHandler struct{}

func (a *DummyHandler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	// Write MatchedApps to http ResponseWriter
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{}"))
}

func (a *DummyHandler) HandleOrgs(w http.ResponseWriter, r *http.Request) {
	// Write MatchedApps to http ResponseWriter
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`
{
  "resources": [
    {
      "guid": "10cdc7f0-6a27-4abf-8c81-691a767886bf",
      "name": "foo"
    }
  ]
}
	`))
}

func (a *DummyHandler) HandleSpaces(w http.ResponseWriter, r *http.Request) {
	// Write MatchedApps to http ResponseWriter
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`
{
  "resources": [
    {
      "guid": "40daaeb5-db5c-4389-9cc5-a8406f1b1489",
      "name": "bar"
    }
  ]
}
	`))
}
