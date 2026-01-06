package controller

import (
	"encoding/json"
	"fmt"
	"testing"

	istioiov1api "istio.io/api/networking/v1"
)

func TestMain(t *testing.T) {
	example := []*istioiov1api.IstioEgressListener{}
	e := &istioiov1api.IstioEgressListener{}
	e.Hosts = []string{"./*", "istio-system/*", "ns1/*"}
	example = append(example, e)

	b, err := json.Marshal(example)
	if err != nil {
		fmt.Printf("Error: %s", err)
	}

	t.Log(string(b))
}
