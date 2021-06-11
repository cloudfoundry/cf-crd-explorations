package handlers_test

import (
	"fmt"
	"reflect"
	"testing"
	//"cloudfoundry.org/cf-crd-explorations/cfshim/handlers"
)

func TestQueryParams(t *testing.T) {
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
func TestQueryParamsConvert(t *testing.T) {
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
