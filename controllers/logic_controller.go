/*


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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	databaselogicv1alpha1 "github.com/Minimize-the-app-upgrade-downtime/Database_Operator/api/v1alpha1"
)

// LogicReconciler reconciles a Logic object
type LogicReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=databaselogic.example.com,resources=logics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=databaselogic.example.com,resources=logics/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=databaselogic.example.com,resources=logics/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;
func (r *LogicReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("logic", req.NamespacedName)

	// fetch the spec
	logic := &databaselogicv1alpha1.Logic{}
	err := r.Get(ctx, req.NamespacedName, logic)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("logic not found.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get logic spec")
	}

	// print logic spec
	fmt.Println("Print logic input : ")
	fmt.Printf("Size : %d\n", logic.Spec.Size)
	fmt.Printf("AppVersion : %s\n", logic.Spec.AppVersion)
	fmt.Printf("Database Version : %s\n", logic.Spec.DatabaseVersion)
	fmt.Printf("IsUpdated : %t\n", logic.Spec.IsUpdated)
	fmt.Printf("Image : %s\n", logic.Spec.Image)
	fmt.Printf("SideCarImage : %s\n", logic.Spec.SideCarImage)
	fmt.Printf("SideCarIsUpdated : %t\n", logic.Spec.SideCarIsUpdated)
	fmt.Printf("Request Count : %d\n", logic.Spec.RequestCount)
	fmt.Printf("Stutus avilable replicas : %d\n", logic.Status.AvailableReplicas)
	fmt.Printf("Status pod name : %s\n", logic.Status.PodNames)
	fmt.Printf("Name : %s\n", logic.Name)
	fmt.Printf("NameSpace : %s\n", logic.Namespace)

	return ctrl.Result{}, nil
}

func (r *LogicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databaselogicv1alpha1.Logic{}).
		Complete(r)
}
