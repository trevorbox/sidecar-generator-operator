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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	"net/http"

	"crypto/sha256"

	networkingv1alpha1 "github.com/trevorbox/sidecar-generator-operator/api/v1alpha1"
	istioiov1api "istio.io/api/networking/v1"
	istioiov1 "istio.io/client-go/pkg/apis/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// SidecarGeneratorReconciler reconciles a SidecarGenerator object
type SidecarGeneratorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=networking.example.com,resources=sidecargenerators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.example.com,resources=sidecargenerators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.example.com,resources=sidecargenerators/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.istio.io,resources=sidecars,verbs=get;list;watch;create;update;patch
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

	err := r.Get(ctx, req.NamespacedName, instance)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			// log.Info("SidecarGenerator resource not found. Ignoring since object is most likely deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get SidecarGenerator")
		return ctrl.Result{}, err
	}

	targetSidecar, updateErr := generateSidecarFromInstance(instance)
	if updateErr != nil {
		log.Error(updateErr, "Failed to create target sidecar spec from SidecarGenerator instance")
	} else {
		existingSidecar := &istioiov1.Sidecar{}
		getErr := r.Get(ctx, req.NamespacedName, existingSidecar)
		if getErr != nil {
			// Sidecar does not exist - create it
			updateErr = r.Create(ctx, targetSidecar)
		} else {
			// Sidecar exists - update it if the spec has changed
			computedHash, err := computeHash(&existingSidecar.Spec)
			if err != nil {
				log.Error(err, "Failed to compute hash for existing Sidecar spec")
				updateErr = err
				return ctrl.Result{}, updateErr
			}
			if computedHash != targetSidecar.Annotations[hashAnnotationName] {
				log.Info("Sidecar spec has changed, updating Sidecar", "oldSpec", &existingSidecar.Spec, "newSpec", &targetSidecar.Spec)
				targetSidecar.ResourceVersion = existingSidecar.ResourceVersion
				updateErr = r.Update(ctx, targetSidecar)
			} else {
				log.Info("Sidecar spec is up to date, no update required")
			}
		}

	}

	now := metav1.Now()
	instance.Status.LastSidecarGeneratorUpdate = &metav1.Time{Time: now.Time}
	instance.Status.NextSidecarGeneratorUpdate = &metav1.Time{Time: now.Add(instance.Spec.RefreshPeriod.Duration)}

	if updateErr != nil {
		log.Error(updateErr, "Failed to reconcile SidecarGenerator")
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

	return ctrl.Result{RequeueAfter: instance.Spec.RefreshPeriod.Duration}, err
}

func generateSidecarFromInstance(instance *networkingv1alpha1.SidecarGenerator) (*istioiov1.Sidecar, error) {
	egress, err := getEgress(instance.Spec.URL, instance.Spec.InsecureSkipTLSVerify)
	if err != nil {
		return nil, err
	}
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
	if instance.Spec.WorkloadSelector != nil {
		sidecar.Spec.WorkloadSelector = &istioiov1api.WorkloadSelector{
			Labels: instance.Spec.WorkloadSelector.Labels,
		}
	}

	computedHash, err := computeHash(&sidecar.Spec)
	if err != nil {
		return nil, err
	}
	sidecar.SetAnnotations(
		map[string]string{
			hashAnnotationName: computedHash,
		},
	)
	return sidecar, nil
}

func getEgress(url string, insecureSkipTLSVerify bool) ([]*istioiov1api.IstioEgressListener, error) {
	// 1. Define a httpClient with a timeout (don't use the default httpClient)
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecureSkipTLSVerify,
			},
		},
	}

	// 2. Create a new HTTP GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 3. Add custom headers (optional)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SidecarGeneratorController/1.0")

	// 4. Send the request
	resp, err := httpClient.Do(req)
	if err != nil {
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
		return nil, fmt.Errorf("unexpected status code: %d for url %s", resp.StatusCode, url)
	}

	// 7. Read and process the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 8. Unmarshal the JSON response into the desired struct
	egress := []*istioiov1api.IstioEgressListener{}
	err = json.Unmarshal(body, &egress)
	if err != nil {
		return nil, err
	}

	return egress, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *SidecarGeneratorReconciler) SetupWithManager(mgr ctrl.Manager) error {

	sidecarGeneratorPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			new, ok := e.ObjectNew.DeepCopyObject().(*networkingv1alpha1.SidecarGenerator)
			if !ok {
				return false
			}
			old, ok := e.ObjectOld.DeepCopyObject().(*networkingv1alpha1.SidecarGenerator)
			if !ok {
				return false
			}

			if !new.GetDeletionTimestamp().IsZero() {
				return true
			}

			if !reflect.DeepEqual(old.Spec, new.Spec) {
				return true
			}

			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	sidecarPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			new, ok := e.ObjectNew.DeepCopyObject().(*istioiov1.Sidecar)
			if !ok {
				return false
			}

			if r.isOwnedSidecarByController(new) {
				hash := new.Annotations[hashAnnotationName]
				computedhash, err := computeHash(&new.Spec)
				if err == nil && hash != computedhash {
					return true
				}
			}

			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			secret, ok := e.Object.DeepCopyObject().(*istioiov1.Sidecar)
			if !ok {
				return false
			}
			if r.isOwnedSidecarByController(secret) {
				return true
			}
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.SidecarGenerator{}, builder.WithPredicates(sidecarGeneratorPredicate)).
		Owns(&istioiov1.Sidecar{}, builder.WithPredicates(sidecarPredicate)).
		Named("sidecargenerator").
		Complete(r)
}

const (
	hashAnnotationName   = "sidecar-generator-operator/sidecar-spec-hash"
	sidecarGeneratorKind = "SidecarGenerator"
)

func (r *SidecarGeneratorReconciler) isOwnedSidecarByController(secret *istioiov1.Sidecar) bool {
	for _, ownerRef := range secret.GetOwnerReferences() {
		if ownerRef.Kind == sidecarGeneratorKind {
			return true
		}
	}
	return false
}

func computeHash(spec interface{}) (string, error) {
	// 1. Marshal the spec to JSON
	data, err := json.Marshal(spec)

	if err != nil {
		return "", err
	}

	// 2. Compute SHA256 hash
	hash := sha256.Sum256(data)

	// 3. Return as a hex string
	return fmt.Sprintf("%x", hash), nil
}
