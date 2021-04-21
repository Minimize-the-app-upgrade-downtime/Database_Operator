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
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	intstr "k8s.io/apimachinery/pkg/util/intstr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	databaselogicv1alpha1 "github.com/Minimize-the-app-upgrade-downtime/Database_Operator/api/v1alpha1"
)

// LogicReconciler reconciles a Logic object
type LogicReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=databaselogic.example.com,namespace=default,resources=logics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=databaselogic.example.com,namespace=default,resources=logics/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=databaselogic.example.com,namespace=default,resources=logics/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,namespace=default,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;
func (r *LogicReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("logic", req.NamespacedName)

	// fetch the spec
	logic := &databaselogicv1alpha1.Logic{}
	err := r.Get(ctx, req.NamespacedName, logic)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Logic resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get logic spec")
	}

	// argument shoud be eqaul to odd
	fmt.Println()
	log.Info("Size             : ", " size             : ", logic.Spec.Size)
	log.Info("AppVersion       : ", " AppVersion       : ", logic.Spec.AppVersion)
	log.Info("Database Version : ", " Database Version : ", logic.Spec.DatabaseVersion)
	log.Info("IsUpdated        : ", " IsUpdated        : ", logic.Spec.IsUpdated)
	log.Info("Image            : ", " Image            : ", logic.Spec.Image)
	log.Info("SideCarImage     : ", " SideCarImage     : ", logic.Spec.SideCarImage)
	log.Info("SideCarIsUpdated : ", " SideCarIsUpdated : ", logic.Spec.SideCarIsUpdated)
	log.Info("Request Count    : ", " Request Count    : ", logic.Spec.RequestCount)
	log.Info("Stutus avil.rep  : ", " Stutus avila.rep : ", logic.Status.AvailableReplicas)
	log.Info("Status pod name  : ", " Status pod name  : ", logic.Status.PodNames)
	log.Info("Name             : ", " Name             : ", logic.Name)
	log.Info("NameSpace        : ", " NameSpace        : ", logic.Namespace)

	// Check if the deployment already exists, if not create a new deployment.
	found_dep := &appsv1.Deployment{}
	// print pretty
	b, e := json.MarshalIndent(found_dep, "", "  ")
	if e != nil {
		fmt.Println(e)
	}

	fmt.Println("\nappsv1 deployment (found_dep) Before Deployment : \n", string(b))

	err = r.Get(ctx, types.NamespacedName{Name: logic.Name, Namespace: logic.Namespace}, found_dep)
	fmt.Println("error : ", err)
	if err != nil {
		if errors.IsNotFound(err) {
			// Define and create a new deployment.
			log.Info("Create new object")
			dep := r.deploymentForApp(logic)
			if err = r.Create(ctx, dep); err != nil {
				log.Info("Create new object error")
				return ctrl.Result{}, err
			}
			log.Info("Not found and requeue error")
			return ctrl.Result{Requeue: true}, nil
		} else {
			log.Info("error return")
			return ctrl.Result{}, err
		}
	}

	b, e = json.MarshalIndent(found_dep, "", "  ")
	if e != nil {
		fmt.Println(e)
	}
	fmt.Println("\nappsv1 deployment (found_dep) After Deployment : \n", string(b))

	//service
	fmt.Println()
	fmt.Println()
	found_ser := &corev1.Service{}
	b, e = json.MarshalIndent(found_ser, "", "  ")
	if e != nil {
		fmt.Println(e)
	}
	fmt.Println("corev1 service before create: \n", string(b))

	err = r.Get(ctx, types.NamespacedName{Name: "my-service", Namespace: logic.Namespace}, found_ser)
	fmt.Println("service error", err)
	if err != nil {
		if errors.IsNotFound(err) {
			ser := r.serviceForApp(logic)
			if err = r.Create(ctx, ser); err != nil {
				log.Info("Service object create error")
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		} else {
			return ctrl.Result{}, err
		}

	}

	fmt.Println()
	found_ser = &corev1.Service{}
	b, e = json.MarshalIndent(found_ser, "", "  ")
	if e != nil {
		fmt.Println(e)
	}
	fmt.Println("corev1 service  After create: \n", string(b))

	return ctrl.Result{}, nil
}

func (r *LogicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databaselogicv1alpha1.Logic{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

// deployment go proxy
// deploymentForApp returns a app Deployment object.
func (r *LogicReconciler) deploymentForApp(m *databaselogicv1alpha1.Logic) *appsv1.Deployment {
	//lbls := labelsForApp(m.Name) //
	replicas := m.Spec.Size // size of the replicas

	dep := &appsv1.Deployment{

		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "push-queue",
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "push-queue",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "push-queue",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: "2016csc044/queue:v1",
						Name:  "push-queue",
						Env: []corev1.EnvVar{{
							Name:  "PORT",
							Value: "50000",
						}},
					}},
				},
			},
		},
	}

	// Set App instance as the owner and controller.
	// NOTE: calling SetControllerReference, and setting owner references in
	// general, is important as it allows deleted objects to be garbage collected.
	controllerutil.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// labelsForApp creates a simple set of labels for App.
// func labelsForApp(name string) map[string]string {
// 	return map[string]string{"app": "push-queue"}
// }

//
func (r *LogicReconciler) serviceForApp(m *databaselogicv1alpha1.Logic) *corev1.Service {

	ser := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service",
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "push-queue",
			},
			Type: "NodePort", // Type is ServiceType. Service type is string
			Ports: []corev1.ServicePort{{
				Protocol: "TCP",
				Port:     80,
				//NodePort: 30111,
				TargetPort: intstr.IntOrString{
					IntVal: 50000,
				},
			}},
		},
	}
	controllerutil.SetControllerReference(m, ser, r.Scheme)
	return ser
}
