package settings

import (
	"errors"
	"os"
)

var GlobalSettings *Settings

type Settings struct {
	// RegistryTagBase is the container registry prefix to upload source & build images do
	RegistryTagBase string `json:"registryTagBase"`
}

func Load() (*Settings, error) {
	s := &Settings{}
	var exists bool
	s.RegistryTagBase, exists = os.LookupEnv("REGISTRY_TAG_BASE")
	if !exists {
		return nil, errors.New("REGISTRY_TAG_BASE not configured")
	}

	return s, nil
}
