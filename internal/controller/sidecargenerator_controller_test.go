/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	istioiov1api "istio.io/api/networking/v1"
	istioiov1 "istio.io/client-go/pkg/apis/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1alpha1 "github.com/trevorbox/sidecar-generator-operator/api/v1alpha1"
)

var _ = Describe("SidecarGenerator Controller", func() {
	var (
		server     *ghttp.Server
		statusCode int
		body       []byte
		path       string
		addr       string
	)
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		sidecargenerator := &networkingv1alpha1.SidecarGenerator{}

		BeforeEach(func() {

			server = ghttp.NewServer()
			statusCode = 200
			path = "/"
			example := []*istioiov1api.IstioEgressListener{
				{
					Hosts: []string{"./*", "istio-system/*", "ns1/*"},
				},
			}

			b, errr := json.Marshal(example)
			if errr != nil {
				fmt.Printf("Error: %s", errr)
			}

			body = b
			addr = "http://" + server.Addr() + path
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))
			By("creating the custom resource for the Kind SidecarGenerator")
			err := k8sClient.Get(ctx, typeNamespacedName, sidecargenerator)
			if err != nil && errors.IsNotFound(err) {
				resource := &networkingv1alpha1.SidecarGenerator{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
					Spec: networkingv1alpha1.SidecarGeneratorSpec{
						URL: addr,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &networkingv1alpha1.SidecarGenerator{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance SidecarGenerator")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &SidecarGeneratorReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
			resource := &istioiov1.Sidecar{}
			k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(resource.Spec.Egress).To(HaveLen(1))
			resourceEgressJSON, err := json.Marshal(resource.Spec.Egress)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(resourceEgressJSON)).To(MatchJSON(string(body)))
		})
	})
})
