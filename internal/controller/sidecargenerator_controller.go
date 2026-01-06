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
	"io"

	"net/http"

	networkingv1alpha1 "github.com/trevorbox/sidecar-generator-operator/api/v1alpha1"
	istioiov1api "istio.io/api/networking/v1"
	istioiov1 "istio.io/client-go/pkg/apis/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// SidecarGeneratorReconciler reconciles a SidecarGenerator object
type SidecarGeneratorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=networking.example.com,resources=sidecargenerators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.example.com,resources=sidecargenerators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.example.com,resources=sidecargenerators/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.istio.io,resources=sidecars,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.istio.io,resources=sidecars/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SidecarGenerator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *SidecarGeneratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	instance := &networkingv1alpha1.SidecarGenerator{}

	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		// Handle not found error
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	response, err := http.Get(instance.Spec.URL)

	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error(err, "Failed to read response body")

	}
	defer response.Body.Close()

	egress := []*istioiov1api.IstioEgressListener{}

	fmt.Printf("Response Body: %s\n", string(body))
	log.Info(fmt.Sprintf("Response Body: %s", string(body)))

	err = json.Unmarshal(body, &egress)
	if err != nil {
		log.Error(err, "Failed to unmarshal JSON")
	}

	sidecar := &istioiov1.Sidecar{}
	err = r.Get(ctx, req.NamespacedName, sidecar)
	if err != nil {
		log.Info("There is no existing Sidecar resource, creating one...")

		sidecar := &istioiov1.Sidecar{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "networking.istio.io/v1",
				Kind:       "Sidecar",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Name,
				Namespace: instance.Namespace,
			},
			Spec: istioiov1api.Sidecar{
				Egress: egress,
			},
		}

		r.Create(ctx, sidecar)
		// Handle not found error
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SidecarGeneratorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.SidecarGenerator{}).
		Named("sidecargenerator").
		Complete(r)
}
