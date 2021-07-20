package settings

import (
	"errors"
	"os"
)

var GlobalSettings *Settings

type Settings struct {
	// RegistryTagBase is the container registry prefix to upload source & build images do
	RegistryTagBase         string `json:"registryTagBase"`
	PackageRegistryTagBase  string
	PackageRegistryUsername string
	PackageRegistryPassword string
}

func Load() (*Settings, error) {
	s := &Settings{}
	var exists bool
	s.RegistryTagBase, exists = os.LookupEnv("REGISTRY_TAG_BASE")
	if !exists {
		return nil, errors.New("REGISTRY_TAG_BASE not configured")
	}

	s.PackageRegistryTagBase, exists = os.LookupEnv("PACKAGE_REGISTRY_TAG_BASE")
	if !exists {
		return nil, errors.New("PACKAGE_REGISTRY_TAG_BASE not configured")
	}

	s.PackageRegistryUsername, exists = os.LookupEnv("PACKAGE_REGISTRY_USERNAME")
	if !exists {
		return nil, errors.New("PACKAGE_REGISTRY_USERNAME not configured")
	}

	s.PackageRegistryPassword, exists = os.LookupEnv("PACKAGE_REGISTRY_PASSWORD")
	if !exists {
		return nil, errors.New("PACKAGE_REGISTRY_PASSWORD not configured")
	}

	return s, nil
}
