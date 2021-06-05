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
	"net/http"
	"time"

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

	"database/sql"

	_ "github.com/go-sql-driver/mysql"

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
// +kubebuilder:rbac:groups=databaselogic.example.com,namespace=default,resources=logics/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=apps,namespace=default,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;patch;
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
	r.deploymentFunc(ctx, req, logic)

	// config map
	r.configMapFunc(ctx, req, logic)
	//service
	r.serviceFunc(ctx, req, logic)

	// check correct image deploy in the cluster.
	found := &appsv1.Deployment{}
	r.Get(ctx, types.NamespacedName{Name: logic.Name, Namespace: logic.Namespace}, found)

	// check image version
	if found.Spec.Template.Spec.Containers[0].Image != logic.Spec.AppImage {

		// cureent application version
		currentappVersion := found.Spec.Template.Spec.Containers[0].Image

		// request store in a queue
		_, err := http.Get("http://34.70.130.195:30007/cluster_Reconsile_Enable")
		if err != nil {
			fmt.Println("con. error :", err)
		}

		log.Info("Requst store in the Queue.")

		log.Info("Database Updating .....")
		time.Sleep(30 * time.Second)
		// open mysql connection
		db, err := sql.Open("mysql", "u8il24jxufb4n4ty:t5z5jvsyolrqhn9k@tcp(jhdjjtqo9w5bzq2t.cbetxkdyhwsb.us-east-1.rds.amazonaws.com:3306)/m0ky8hn32ov17miq")
		if err != nil {
			panic(err.Error())
		}
		defer db.Close() // close connection
		// call version update sp
		_, err = db.Query("call updateVersion(?,?)", logic.Spec.AppVersion, logic.Spec.DatabaseVersion)
		if err != nil {
			// chanege the CRD
			logic.Spec.AppImage = currentappVersion
			// downgrade database
			_, err := db.Query("call downgradeVersion(?)", currentappVersion)
			if err != nil {
				//panic(err.Error())
				fmt.Println("db downgrade error : ", err)
			}
			return ctrl.Result{}, err
		} else {

			fmt.Println("pod updating...")

			// apply image to container
			patch := client.MergeFrom(found.DeepCopy())
			found.Spec.Template.Spec.Containers[0].Image = logic.Spec.AppImage
			fmt.Println("found updated image before patch : ", found.Spec.Template.Spec.Containers[0].Image)
			err = r.Patch(ctx, found, patch)
			if err != nil {
				panic(err.Error())
			}
			fmt.Println("found updated image after patch : ", found.Spec.Template.Spec.Containers[0].Image)
			if err != nil {
				fmt.Println("patch update error")

				// downgrade database
				_, err := db.Query("call downgradeVersion(?)", currentappVersion)
				if err != nil {
					// panic(err.Error())
					fmt.Println("db downgrade error : ", err)
				}
				// downgrade app
				patch := client.MergeFrom(found.DeepCopy())
				found.Spec.Template.Spec.Containers[0].Image = currentappVersion
				logic.Spec.AppImage = currentappVersion
				err = r.Patch(ctx, found, patch)
				if err != nil {
					fmt.Println("Application Downgrade Error ", err)
				}
				fmt.Println("Patch error reconsile")
				return ctrl.Result{}, err
			} else {

				fmt.Println("Mergin...")
				patch := client.MergeFrom(found.DeepCopy())
				found.Spec.Template.Spec.Containers[1].Image = logic.Spec.SchemaChangeApplyImage
				err := r.Patch(ctx, found, patch)
				if err != nil {
					fmt.Println("Schema change apply error : ", err)
				}
			}
		}

		log.Info("Database and app Successfully Update.")

		// requst pop and give the cluster. cluster is working properly
		http.Get("http://34.70.130.195:30007/cluster_Reconsile_Disable")
		log.Info("Requst pop in the Queue. Clsuter is working properly.")

		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	if found.Spec.Template.Spec.Containers[1].Image != logic.Spec.DefaultSchemaImage {
		patch := client.MergeFrom(found.DeepCopy())
		found.Spec.Template.Spec.Containers[1].Image = logic.Spec.DefaultSchemaImage
		if err = r.Patch(ctx, found, patch); err != nil {
			return ctrl.Result{Requeue: true}, nil
		}
	}

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
	log.Info("Stutus avil.rep  : ", " Stutus avila.rep : ", l.Status.AvailableReplicas)
	log.Info("App Image Name   : ", " App Image Name   : ", l.Spec.AppName)
	log.Info("App Image & Ver  : ", " App Image & Ver  : ", l.Spec.AppImage)
	log.Info("SideCar Name     : ", " SideCar Name     : ", l.Spec.SideCarName)
	log.Info("SideCar Image&Ver: ", " SideCar Image&Ver: ", l.Spec.SideCarImage)
	log.Info("Sch.chan Name    : ", " Sch.chan  Name   : ", l.Spec.SchemaChangeApplyName)
	log.Info("sch.cha Image&Ver: ", " sch.cha Image&Ver: ", l.Spec.SchemaChangeApplyImage)
	log.Info("Status pod name  : ", " Status pod name  : ", l.Status.PodNames)
	log.Info("Name             : ", " Name             : ", l.Name)
	log.Info("NameSpace        : ", " NameSpace        : ", l.Namespace)
	fmt.Println()
}

// deployment
func (r *LogicReconciler) deploymentFunc(ctx context.Context, req ctrl.Request, logic *databaselogicv1alpha1.Logic) (ctrl.Result, error) {

	found_dep := &appsv1.Deployment{}

	err := r.Get(ctx, types.NamespacedName{Name: logic.Name, Namespace: logic.Namespace}, found_dep)

	if err != nil {

		if errors.IsNotFound(err) {

			// Define and create a new deployment for App.
			dep := r.deploymentForApp(logic)
			if err = r.Create(ctx, dep); err != nil {
				log.Error("Deployment App create error :", err)
				return ctrl.Result{}, err
			}

			log.Warn("App Deployment Requeue !")
			return ctrl.Result{Requeue: true}, nil

		} else {
			log.Error("Deployment", err)
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
		log.Error("Deployment Pretty Convert Error : ", err)
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
					"app": m.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": m.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: m.Spec.AppImage,
							Name:  m.Spec.AppName,
						},
						{
							Image: m.Spec.DefaultSchemaImage, // default current version schema
							Name:  m.Spec.SchemaChangeApplyName,
							Env: []corev1.EnvVar{{
								Name:  "PORT",
								Value: "50002",
							}},
						},
						{
							Image: m.Spec.SideCarImage,
							Name:  m.Spec.SideCarName,
							Env: []corev1.EnvVar{{
								Name:  "PORT",
								Value: "50000",
							}},
						},
					},
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
func (r *LogicReconciler) serviceFunc(ctx context.Context, req ctrl.Request, logic *databaselogicv1alpha1.Logic) (ctrl.Result, error) {

	found_ser := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: logic.Name, Namespace: logic.Namespace}, found_ser)
	if err != nil {
		if errors.IsNotFound(err) {
			ser := r.serviceForApp(logic)
			if err = r.Create(ctx, ser); err != nil {
				log.Error("Service create error : ", err)
				return ctrl.Result{}, err
			}
			log.Warn("Service Create Requeue!")
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
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": m.Name,
			},
			Type: "NodePort", // ServiceType : LoadBalancer,NodePort
			Ports: []corev1.ServicePort{{
				Protocol: "TCP",
				Port:     80,
				NodePort: 30007,
				TargetPort: intstr.IntOrString{
					IntVal: 50000,
				},
			}},
		},
	}
	controllerutil.SetControllerReference(m, ser, r.Scheme)
	return ser
}

func (r *LogicReconciler) configMapFunc(ctx context.Context, req ctrl.Request, logic *databaselogicv1alpha1.Logic) (ctrl.Result, error) {

	found_config := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: logic.Name, Namespace: logic.Namespace}, found_config)
	if err != nil {
		if errors.IsNotFound(err) {
			con := r.configMapForApp(logic)
			if err = r.Create(ctx, con); err != nil {
				log.Error("Configmap create error : ", err)
				return ctrl.Result{}, err
			}

			log.Warn("Config Map  Requeue!")
			return ctrl.Result{Requeue: true}, nil
		} else {
			return ctrl.Result{}, err
		}

	}
	printConfigMap(found_config)
	return ctrl.Result{}, nil
}

// print ConfigMap pretty
func printConfigMap(con *corev1.ConfigMap) {
	b, err := json.MarshalIndent(con, "", "  ")
	if err != nil {
		log.Error("ConfigMap Pretty Convert Error : ", err)
	}
	fmt.Println()
	fmt.Println("ConfigMap : \n", string(b))
}

// config yaml
func (r *LogicReconciler) configMapForApp(m *databaselogicv1alpha1.Logic) *corev1.ConfigMap {

	con := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Data: map[string]string{
			"epfDetails.conf": "server { location / { proxy_pass http://localhost:3000; } location / { proxy_pass http://localhost:50002; }   }",
		},
	}
	controllerutil.SetControllerReference(m, con, r.Scheme)
	return con
}
