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
	"time"

	"net/http"

	networkingv1alpha1 "github.com/trevorbox/sidecar-generator-operator/api/v1alpha1"
	istioiov1api "istio.io/api/networking/v1"
	istioiov1 "istio.io/client-go/pkg/apis/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("SidecarGenerator resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get SidecarGenerator")
		return ctrl.Result{}, err
	}

	egress, updateErr := getEgress(ctx, instance.Spec.URL)
	// egress, updateErr := getTestEgress()
	if updateErr != nil {
		log.Error(updateErr, "Failed to get egress from URL")
	} else {
		sidecar := &istioiov1.Sidecar{}
		getErr := r.Get(ctx, req.NamespacedName, sidecar)
		if getErr != nil {
			log.Info("There is no existing Sidecar resource, creating one...")

			sidecar := &istioiov1.Sidecar{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "networking.istio.io/v1",
					Kind:       "Sidecar",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.Name,
					Namespace: instance.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(instance, instance.GroupVersionKind()),
					},
				},
				Spec: istioiov1api.Sidecar{
					Egress: egress,
				},
			}

			updateErr = r.Create(ctx, sidecar)
			if updateErr == nil {
				log.Info("Created Sidecar")
			}

		} else {
			log.Info("Updating existing Sidecar resource...")
			sidecar.Spec.Egress = egress
			updateErr = r.Update(ctx, sidecar)
			if updateErr == nil {
				log.Info("Updated Sidecar")
			}
		}
	}

	now := metav1.Now()
	instance.Status.LastSidecarGeneratorUpdate = &metav1.Time{Time: now.Time}
	instance.Status.NextSidecarGeneratorUpdate = &metav1.Time{Time: now.Add(instance.Spec.RefreshPeriod.Duration)}

	if updateErr != nil {
		instance.Status.Conditions = []metav1.Condition{
			{
				Type:               "Reconciled",
				Status:             metav1.ConditionFalse,
				Reason:             "ReconcileError",
				Message:            fmt.Sprintf("Failed to reconcile SidecarGenerator: %v", updateErr),
				LastTransitionTime: now,
			},
		}

	} else {
		instance.Status.Conditions = []metav1.Condition{
			{
				Type:               "Reconciled",
				Status:             metav1.ConditionTrue,
				Reason:             "ReconcileSuccess",
				Message:            "Successfully reconciled SidecarGenerator",
				LastTransitionTime: now,
			},
		}
	}

	err = r.Status().Update(ctx, instance)
	if err != nil {
		log.Error(err, "Failed to update SidecarGenerator status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true, RequeueAfter: instance.Spec.RefreshPeriod.Duration}, err
}

func getEgress(ctx context.Context, url string) ([]*istioiov1api.IstioEgressListener, error) {
	log := logf.FromContext(ctx)

	log.Info("Fetching egress from URL", "url", url)

	// 1. Define a client with a timeout (don't use the default client)
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 2. Create the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error(err, "Error creating request", "url", url)
		return nil, err
	}

	// 3. Add custom headers (optional)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SidecarGeneratorController/1.0")

	// 4. Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err, "Failed to send HTTP request to URL", "url", url)
		return nil, err
	}

	// 5. IMPORTANT: Close the body when the function exits
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// 6. Check the status code
	if resp.StatusCode != http.StatusOK {
		log.Error(fmt.Errorf("unexpected status code: %d", resp.StatusCode), "Status not OK", "url", url)
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 7. Read and process the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error(err, "Failed to read response body from URL", "url", url)
		return nil, err
	}

	egress := []*istioiov1api.IstioEgressListener{}

	err = json.Unmarshal(body, &egress)
	if err != nil {
		log.Error(err, "Failed to unmarshal response body from URL", "url", url)
		return nil, err
	}

	return egress, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *SidecarGeneratorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.SidecarGenerator{}).
		Named("sidecargenerator").
		Complete(r)
}
