package controller

import (
	"encoding/json"
	"fmt"
	"testing"

	v1alpha3 "istio.io/api/networking/v1alpha3"
)

func TestMain(t *testing.T) {
	example := []*v1alpha3.IstioEgressListener{}
	e := &v1alpha3.IstioEgressListener{}
	e.Hosts = []string{"./*", "istio-system/*", "ns1/*"}
	example = append(example, e)

	b, err := json.Marshal(example)
	if err != nil {
		fmt.Printf("Error: %s", err)
	}

	t.Log(string(b))
}
