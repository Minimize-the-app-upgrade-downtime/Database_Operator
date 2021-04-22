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
	"github.com/prometheus/common/log"
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

	// get the Logic
	logic, err := r.getLogicAPI(ctx, req)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Logic resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Logic")
		return ctrl.Result{}, err
	}
	// print Logic API Value
	r.logicAPIValuePrint(logic)

	// Create deploment
	r.deploymentProxyApp(ctx, req, logic)

	//service
	r.serviceProxyApp(ctx, req, logic)

	return ctrl.Result{}, nil
}

// watch the resource
func (r *LogicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databaselogicv1alpha1.Logic{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

// Check Logic API
func (r *LogicReconciler) getLogicAPI(ctx context.Context, req ctrl.Request) (*databaselogicv1alpha1.Logic, error) {

	logic := &databaselogicv1alpha1.Logic{}
	err := r.Get(ctx, req.NamespacedName, logic)
	return logic, err
}

// Print logic API
func (r *LogicReconciler) logicAPIValuePrint(l *databaselogicv1alpha1.Logic) {

	log := r.Log.WithName("Logic CRD : ")
	fmt.Println()
	fmt.Println("Print Logic Resource Values : ")
	log.Info("Size             : ", " size             : ", l.Spec.Size)
	log.Info("AppVersion       : ", " AppVersion       : ", l.Spec.AppVersion)
	log.Info("Database Version : ", " Database Version : ", l.Spec.DatabaseVersion)
	log.Info("IsUpdated        : ", " IsUpdated        : ", l.Spec.IsUpdated)
	log.Info("Image            : ", " Image            : ", l.Spec.Image)
	log.Info("SideCarImage     : ", " SideCarImage     : ", l.Spec.ProxyImage)
	log.Info("SideCarIsUpdated : ", " SideCarIsUpdated : ", l.Spec.SideCarIsUpdated)
	log.Info("Request Count    : ", " Request Count    : ", l.Spec.RequestCount)
	log.Info("Stutus avil.rep  : ", " Stutus avila.rep : ", l.Status.AvailableReplicas)
	log.Info("Status pod name  : ", " Status pod name  : ", l.Status.PodNames)
	log.Info("Name             : ", " Name             : ", l.Name)
	log.Info("NameSpace        : ", " NameSpace        : ", l.Namespace)
	fmt.Println()
}

// deployment
func (r *LogicReconciler) deploymentProxyApp(ctx context.Context, req ctrl.Request, logic *databaselogicv1alpha1.Logic) (ctrl.Result, error) {

	found_dep := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: logic.Name, Namespace: logic.Namespace}, found_dep)
	if err != nil {
		if errors.IsNotFound(err) {
			// Define and create a new deployment.
			dep := r.deploymentForApp(logic)
			if err = r.Create(ctx, dep); err != nil {
				log.Info("Deployment create error", err)
				return ctrl.Result{}, err
			}
			log.Warn("Deployment Requeue")
			return ctrl.Result{Requeue: true}, nil
		} else {
			log.Info("Deployment Get Error", err)
			return ctrl.Result{}, err
		}
	}
	printDeployment(found_dep)
	return ctrl.Result{}, nil
}

// print deployment pretty
func printDeployment(dep *appsv1.Deployment) {
	b, err := json.MarshalIndent(dep, "", "  ")
	if err != nil {
		log.Error("Service Pretty Convert Error : ", err)
	}
	fmt.Println()
	fmt.Println("Deployment : \n", string(b))
}

// deploymentForApp returns a app Deployment object.
func (r *LogicReconciler) deploymentForApp(m *databaselogicv1alpha1.Logic) *appsv1.Deployment {

	replicas := m.Spec.Size // size of the replicas

	dep := &appsv1.Deployment{

		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
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
						Image: m.Spec.ProxyImage,
						Name:  "side-car",
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

// service for proxy
func (r *LogicReconciler) serviceProxyApp(ctx context.Context, req ctrl.Request, logic *databaselogicv1alpha1.Logic) (ctrl.Result, error) {

	found_ser := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: "my-service", Namespace: logic.Namespace}, found_ser)
	if err != nil {
		if errors.IsNotFound(err) {
			ser := r.serviceForApp(logic)
			if err = r.Create(ctx, ser); err != nil {
				log.Error("proxy service create error : ", err)
				return ctrl.Result{}, err
			}
			log.Warn("Proxy Service Create Requeue!")
			return ctrl.Result{Requeue: true}, nil
		} else {
			return ctrl.Result{}, err
		}

	}
	printService(found_ser)
	return ctrl.Result{}, nil
}

// print Serviec pretty
func printService(ser *corev1.Service) {
	b, err := json.MarshalIndent(ser, "", "  ")
	if err != nil {
		log.Error("Service Pretty Convert Error : ", err)
	}
	fmt.Println()
	fmt.Println("Service : \n", string(b))
}

// service yaml
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
			Type: "NodePort", // ServiceType : LoadBalancer,NodePort
			Ports: []corev1.ServicePort{{
				Protocol: "TCP",
				Port:     80,
				NodePort: 30111,
				TargetPort: intstr.IntOrString{
					IntVal: 50000,
				},
			}},
		},
	}
	controllerutil.SetControllerReference(m, ser, r.Scheme)
	return ser
}
