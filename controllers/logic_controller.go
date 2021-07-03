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
	"net/http"
	"time"

	"github.com/go-logr/logr"
	//"github.com/gobuffalo/flect/name"
	//"github.com/prometheus/common/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	//intstr "k8s.io/apimachinery/pkg/util/intstr"

	"database/sql"

	databaselogicv1alpha1 "github.com/Minimize-the-app-upgrade-downtime/Database_Operator/api/v1alpha1"
	_ "github.com/go-sql-driver/mysql"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

	// check the API
	logic := &databaselogicv1alpha1.Logic{}
	err := r.Get(ctx, req.NamespacedName, logic)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Logic resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, " : Fail to get Logic")
	}
	// create deployment
	err = r.deploymentFunc(ctx, req, logic)
	if err != nil {
		log.Error(err, "App , proxy, schema converter Deployment failed.")
	} else {
		log.Info("Deployment Succesfully !!..")
	}
	// create service
	err = r.serviceFunc(ctx, req, logic)
	if err != nil {
		log.Error(err, "App , proxy  Service failed.")
	} else {
		log.Info("Service Succesfully !!..")
	}
	// handle update
	dep := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: "app-dep", Namespace: logic.Namespace}, dep)
	if err != nil {
		log.Error(err, "404 , Deployment not found!!")
	}
	if dep.Spec.Template.Spec.Containers[0].Image != logic.Spec.AppImage {
		err = r.UpdataApp(dep, logic)
		if err != nil {
			log.Error(err, "Application update error!!")
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

func (r *LogicReconciler) UpdataApp(dep *appsv1.Deployment, logic *databaselogicv1alpha1.Logic) error {

	db, err := sql.Open("mysql", "u8il24jxufb4n4ty:t5z5jvsyolrqhn9k@tcp(jhdjjtqo9w5bzq2t.cbetxkdyhwsb.us-east-1.rds.amazonaws.com:3306)/m0ky8hn32ov17miq")
	if err != nil {
		r.Log.Error(err, "Database connection open error!!.")
		return err
	}
	defer db.Close()
	// store the request in the queue
	_, err = http.Get("http://34.95.82.187/updateStarted")
	if err != nil {
		r.Log.Error(err, "Request is not stored in the queue.")
		return err
	}
	// update database
	downgradeAppVersion := dep.Spec.Template.Spec.Containers[0].Image
	_, err = db.Query("call updateVersion(?,?)", logic.Spec.AppVersion, logic.Spec.DatabaseVersion)
	if err != nil {
		r.Log.Error(err, "Database Update error !!")
		r.downgradeDB(downgradeAppVersion, db)
		r.downgradePod(dep, logic, downgradeAppVersion)
		r.stopRequestStore()
		return err
	}
	time.Sleep(30 * time.Second)
	r.Log.Info("Updating........................................")
	// update application
	dep.Spec.Template.Spec.Containers[0].Image = logic.Spec.AppImage
	dep.Spec.Template.Spec.Containers[1].Image = logic.Spec.SchemaCovertorImage
	ctx := context.TODO()
	err = r.Update(ctx, dep)
	if err != nil {
		r.Log.Error(err, "Pod Update failure")
		r.downgradeDB(downgradeAppVersion, db)
		r.downgradePod(dep, logic, downgradeAppVersion)
		r.stopRequestStore()
		return err
	}
	r.stopRequestStore()

	return err
}

func (r *LogicReconciler) downgradePod(dep *appsv1.Deployment, logic *databaselogicv1alpha1.Logic, ver string) {
	dep.Spec.Template.Spec.Containers[0].Image = ver
	dep.Spec.Template.Spec.Containers[1].Image = logic.Spec.DefaultSchemaImage
	ctx := context.TODO()
	err := r.Update(ctx, dep)
	if err != nil {
		r.Log.Error(err, " App downgrade error!! ")
	}
}

// downgrade db version
func (r *LogicReconciler) downgradeDB(ver string, db *sql.DB) {
	_, err := db.Query("call downgradeVersion(?)", ver) // downgrade dataabase if failure
	if err != nil {
		r.Log.Error(err, "Database downgrade error !!")
	}
}

// stop request store
func (r *LogicReconciler) stopRequestStore() {
	// store the request in the queue
	_, err := http.Get("http://34.95.82.187/updateFinished")
	if err != nil {
		r.Log.Error(err, "Request Store Stop!!.")
	}
}

func (r *LogicReconciler) serviceFunc(ctx context.Context, req ctrl.Request, logic *databaselogicv1alpha1.Logic) error {
	check_ser := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: "app-svc", Namespace: logic.Namespace}, check_ser)
	if err != nil {
		if errors.IsNotFound(err) {
			svc := r.serviceForApp(logic)
			if err := r.Create(ctx, svc); err != nil {
				r.Log.Error(err, "App service create error !!.")
				return err
			}
		} else {
			return err
		}
	}

	err = r.Get(ctx, types.NamespacedName{Name: "proxy-svc", Namespace: logic.Namespace}, check_ser)
	if err != nil {
		if errors.IsNotFound(err) {
			svc := r.serviceForProxy(logic)
			if err := r.Create(ctx, svc); err != nil {
				r.Log.Error(err, "Proxy service create error !!.")
				return err
			}
		} else {
			return err
		}
	}
	return err
}

func (r *LogicReconciler) serviceForApp(m *databaselogicv1alpha1.Logic) *corev1.Service {

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-svc",
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "backend",
			},
			Ports: []corev1.ServicePort{
				{
					Protocol:   "TCP",
					Name:       "app-port",
					Port:       3000,
					TargetPort: intstr.IntOrString{IntVal: 3000},
				},
				{
					Protocol:   "TCP",
					Name:       "schema-port",
					Port:       50002,
					TargetPort: intstr.IntOrString{IntVal: 50002},
				},
			},
		},
	}
	controllerutil.SetControllerReference(m, svc, r.Scheme)
	return svc
}

func (r *LogicReconciler) serviceForProxy(m *databaselogicv1alpha1.Logic) *corev1.Service {

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy-svc",
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "proxy",
			},
			Type: "NodePort",
			Ports: []corev1.ServicePort{
				{
					Name:       "proxy-port",
					Port:       80,
					NodePort:   30009,
					Protocol:   "TCP",
					TargetPort: intstr.IntOrString{IntVal: 50000},
				},
			},
		},
	}
	controllerutil.SetControllerReference(m, svc, r.Scheme)
	return svc
}

func (r *LogicReconciler) deploymentFunc(ctx context.Context, req ctrl.Request, logic *databaselogicv1alpha1.Logic) error {

	check_dep := appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: "app-dep", Namespace: logic.Namespace}, &check_dep)
	if err != nil {
		if errors.IsNotFound(err) {
			dep := r.deploymentForApp(logic)
			if err = r.Create(ctx, dep); err != nil {
				r.Log.Error(err, " Application deployment error ")
				return err
			}
		} else {
			return err
		}
	}

	err = r.Get(ctx, types.NamespacedName{Name: "proxy-dep", Namespace: logic.Namespace}, &check_dep)

	if err != nil {
		if errors.IsNotFound(err) {
			dep := r.deploymentForProxy(logic)
			if err = r.Create(ctx, dep); err != nil {
				r.Log.Error(err, " Proxy deployment error ")
				return err
			}
		} else {
			return err
		}
	}
	return err
}

func (r *LogicReconciler) deploymentForApp(m *databaselogicv1alpha1.Logic) *appsv1.Deployment {

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-dep",
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &m.Spec.AppReplica,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "backend",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "backend",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  m.Spec.AppName,
							Image: m.Spec.AppImage,
							Ports: []corev1.ContainerPort{
								{
									Name:          "app-port",
									ContainerPort: 3000,
								},
							},
						},
						{
							Name:  m.Spec.SchemaCovertorName,
							Image: m.Spec.DefaultSchemaImage,
							Ports: []corev1.ContainerPort{
								{
									Name:          "schema-port",
									ContainerPort: 50002,
								},
							},
						},
					},
				},
			},
		},
	}
	controllerutil.SetControllerReference(m, dep, r.Scheme)
	return dep
}

func (r *LogicReconciler) deploymentForProxy(m *databaselogicv1alpha1.Logic) *appsv1.Deployment {

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy-dep",
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &m.Spec.ProxyReplica,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "proxy",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "proxy",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  m.Spec.ProxyName,
							Image: m.Spec.ProxyImage,
							Ports: []corev1.ContainerPort{
								{
									Name:          "proxy-port",
									ContainerPort: 50000,
								},
							},
						},
					},
				},
			},
		},
	}
	controllerutil.SetControllerReference(m, dep, r.Scheme)
	return dep
}
