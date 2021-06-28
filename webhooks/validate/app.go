package validate

import (
	"cloudfoundry.org/cf-crd-explorations/cfshim/filters"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	"net/http"
)

/*
		For how to configure the Webhook with kubeapi
	    See: https://docs.giantswarm.io/advanced/custom-admission-controller/
*/
type AppValidator struct {
	// This is a Kuberentes client, contains authentication and context stuff for running K8s queries
	KubeClient client.Client
}

type Filter interface {
	// Filter takes an object, casts it uses preset filters and returns yes/no
	Filter(interface{}) bool
}

func (a *AppValidator) AppValidation(w http.ResponseWriter, r *http.Request) {
	fmt.Println("validation App name is unique...")
	var appRequest *appsv1alpha1.App
	arRequest := v1.AdmissionReview{}

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if err := json.Unmarshal(body, &arRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode("Bad Request")
		return
	}

	if err := json.Unmarshal(arRequest.Request.Object.Raw, &appRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode("Bad Request")
		fmt.Println("error deserializing - Bad Request")
		return
	}

	appName := appRequest.Spec.Name
	//appNamespace := appRequest.GetNamespace()

	queryParameters := map[string][]string{
		"names": {appName},
	}

	var filter Filter = &filters.AppFilter{
		QueryParameters: queryParameters,
	}

	fmt.Println("********* About to make a GET request *******************")

	AllApps := &appsv1alpha1.AppList{}
	err := a.KubeClient.List(context.Background(), AllApps)

	if err != nil {
		fmt.Errorf("error fetching app: %v", err)
	}

	fmt.Printf("******************** Fetching all Apps in default namespace %v", AllApps)

	// Apply filter to AllApps and store result in matchedApps
	var matchedApps []*appsv1alpha1.App
	fmt.Printf("%v\n", matchedApps)
	for i, _ := range AllApps.Items {
		if filter.Filter(&AllApps.Items[i]) {
			matchedApps = append(matchedApps, &AllApps.Items[i])
		}
	}

	fmt.Printf("matched apps : %v", matchedApps)

	if len(matchedApps) == 0 {
		//just returning works here
		//also an other option would be to return AdmissionResponse with Allowed set to true
		return
	}

	arResponse := v1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &v1.AdmissionResponse{
			UID:     arRequest.Request.UID,
			Allowed: false,
			Result: &metav1.Status{
				Message: "App with the name already exists!",
			},
		},
	}

	resp, err := json.Marshal(&arResponse)
	if err != nil {
		fmt.Println("Can't encode response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(fmt.Sprintf("could not encode response: %v", err))
		return
	}
	fmt.Printf("Ready to write reponse ...: %v \n", string(resp))
	if _, err := w.Write(resp); err != nil {
		fmt.Println("Can't write response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(fmt.Sprintf("could not write response: %v", err))
		return
	}
}
