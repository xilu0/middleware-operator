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
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	databasesv1 "github.com/xilu0/middleware-operator/apis/databases/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// MysqlReconciler reconciles a Mysql object
type MysqlReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=databases.wise2c.com,resources=mysqls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=databases.wise2c.com,resources=mysqls/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get;list;watch;create;update;patch;delete

func (r *MysqlReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("mysql", req.NamespacedName)
	r.Log.Info("starting reconlile....")
	mysql := &databasesv1.Mysql{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, mysql)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	foundDeploy := &appsv1.Deployment{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Namespace: mysql.Namespace, Name: mysql.Name}, foundDeploy)
	if err != nil && errors.IsNotFound(err) {
		r.Log.Info("no found, creating deployment for mysql")
		deploy := r.DeploymentForMysql(mysql)
		fmt.Println(deploy)
		err := r.Client.Create(context.TODO(), deploy)
		if err != nil {
			r.Log.Error(err, "failed to create deployment for mysql")
			return ctrl.Result{}, err
		}
	}
	foundService := &corev1.Service{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Namespace: mysql.Namespace, Name: mysql.Name}, foundService)
	if err != nil && errors.IsNotFound(err) {
		r.Log.Info("no found mysql service, createing")
		service := r.ServiceFormysql(mysql)
		err = r.Client.Create(context.TODO(), service)
		if err != nil {
			r.Log.Error(err, "failed to create service for mysql")
			return ctrl.Result{}, err
		}
	}

	// your logic here

	return ctrl.Result{}, nil
}
func (r *MysqlReconciler) DeploymentForMysql(mysql *databasesv1.Mysql) *appsv1.Deployment {
	ls := lables(mysql.Name, mysql.Kind)
	name := strings.ToLower(mysql.Name)
	kind := strings.ToLower(mysql.Kind)
	image := mysql.Spec.Image
	password := mysql.Spec.RootPassword
	affinity := mysql.Spec.Affinity
	hostPath := mysql.Spec.VolumePath
	var size int32 = 1
	var network bool = false
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: mysql.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Replicas: &size,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: image,
						Name:  kind,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 3306,
							Name:          kind,
							Protocol:      "TCP",
						}},
						Env: []corev1.EnvVar{{
							Name:  "MYSQL_ROOT_PASSWORD",
							Value: password,
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "data",
							MountPath: "/var/lib/mysql",
						}},
					}},
					HostNetwork: network,
					Affinity:    &affinity,
					// Affinity: &corev1.Affinity{
					// 	NodeAffinity: &corev1.NodeAffinity{
					// 		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					// 			NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					// 				MatchExpressions: []corev1.NodeSelectorRequirement{{
					// 					Key: "kubernetes.io/hostname",
					// 					Operator: "in",
					// 					Values: []string{
					// 						node,
					// 					},
					// 				}},
					// 			}},
					// 		},
					// 	},
					// },
					Volumes: []corev1.Volume{{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: hostPath,
							},
						},
					}},
				},
			},
		},
	}
	controllerutil.SetControllerReference(mysql, deploy, r.Scheme)
	return deploy
}

func (r *MysqlReconciler) ServiceFormysql(mysql *databasesv1.Mysql) *corev1.Service {
	ls := lables(mysql.Name, mysql.Kind)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: mysql.Namespace,
			Name:      mysql.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{{
				Name:     "mysql",
				Protocol: "TCP",
				Port:     3306,
				TargetPort: intstr.IntOrString{
					Type:   0,
					IntVal: 3306,
					StrVal: "3306",
				},
			}},
			Type: "NodePort",
		},
	}
	controllerutil.SetControllerReference(mysql, service, r.Scheme)
	return service
}

func (r *MysqlReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasesv1.Mysql{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func lables(name string, cr string) map[string]string {
	return map[string]string{"name": name, "cr": cr}
}
