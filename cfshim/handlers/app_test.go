package handlers_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"cloudfoundry.org/cf-crd-explorations/cfshim/handlers"
)

func FTestQueryParams(t *testing.T) {
	queryParams := map[string][]string{
		"0": []string{"a"},
		"1": []string{"a,b"},
		"2": []string{"a", "b", "c"},
	}
	queryParams2 := map[string][]string{
		"0": []string{"a"},
		"1": []string{"a,b"},
		"2": []string{"a", "b", "c"},
	}
	fmt.Printf("qP1: %v", queryParams)
	fmt.Printf("qP2: %v", queryParams2)
	if !reflect.DeepEqual(queryParams, queryParams2) {
		t.Errorf("Error matching")
	}
}
func FTestQueryParamsConvert(t *testing.T) {
	queryParams := map[string][]string{
		"0": []string{"a"},
		"1": []string{"a,b"},
		"2": []string{"a", "b", "c"},
	}
	queryParams2 := map[string][]string{
		"0": []string{"a"},
		"1": []string{"a,b"},
		"2": []string{"a", "b", "c"},
	}
	//handlers.FormatQueryParams(queryParams2)
	fmt.Printf("qP1: %v", queryParams)
	fmt.Printf("qP2: %v", queryParams2)
	if !reflect.DeepEqual(queryParams, queryParams2) {
		t.Errorf("Error matching")
	}
}

func FTestPresenterFormatting(t *testing.T) {
	// Create empty CFAPIPresenterPackageResource
	emptyCFAPIPresenterPackageResource := handlers.CFAPIPresenterPackageResource{}
	emptyCFAPIPresenterPackageResource.Data.Type = "bits"
	//emptyCFAPIPresenterPackageResource.Data.Type = "docker"
	emptyCFAPIPresenterPackageResource.Data.Image = "\"registry/image:latest\""
	emptyCFAPIPresenterPackageResource.Data.Checksum = &handlers.CFAPIPresenterChecksum{}
	emptyCFAPIPresenterPackageResource.Links = make(map[string]handlers.CFAPIAppLink, 0)
	fmt.Printf("%+v\n", emptyCFAPIPresenterPackageResource)
	formattedJSON, _ := json.MarshalIndent(emptyCFAPIPresenterPackageResource, "", "    ")
	fmt.Printf("%+v\n", string(formattedJSON))

	//unformattedJSON, _ := json.Marshal(emptyCFAPIPresenterPackageResource)
	//fmt.Printf("%+v\n", string(unformattedJSON))

}
