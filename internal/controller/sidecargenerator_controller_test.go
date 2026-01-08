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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	istioiov1api "istio.io/api/networking/v1"
	istioiov1 "istio.io/client-go/pkg/apis/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

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
	timeout := time.Second * 60
	interval := time.Second * 2
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
			server.RouteToHandler("GET", path,
				ghttp.RespondWithPtr(&statusCode, &body))
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

			// Close() should be called at the end of each test. It spins down and cleans up the test server.
			server.Close()
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")

			sidecargenerator := &networkingv1alpha1.SidecarGenerator{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, sidecargenerator)).To(Succeed())
				g.Expect(sidecargenerator.Status.Conditions).ToNot(BeEmpty())
			}, timeout, interval).Should(Succeed())

			Expect(sidecargenerator.Status.Conditions).ToNot(BeEmpty())
			Expect(sidecargenerator.Status.Conditions[0].Type).To(Equal("Reconciled"))
			Expect(sidecargenerator.Status.LastSidecarGeneratorUpdate).ToNot(BeNil())
			Expect(sidecargenerator.Status.NextSidecarGeneratorUpdate).ToNot(BeNil())

			By("Verifying the Sidecar exists and has correct Egress")

			resource := &istioiov1.Sidecar{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				g.Expect(resource.Spec.Egress).ToNot(BeEmpty())
			}, timeout, interval).Should(Succeed())

			Expect(resource.Spec.Egress).To(HaveLen(1))
			resourceEgressJSON, err := json.Marshal(resource.Spec.Egress)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(resourceEgressJSON)).To(MatchJSON(string(body)))
		})
	})
})
