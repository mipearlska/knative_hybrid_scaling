/*
Copyright 2023 mipearlska.

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
	"flag"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servingv1client "knative.dev/serving/pkg/client/clientset/versioned/typed/serving/v1"

	hybridscalingv1 "github.com/mipearlska/knative_hybrid_scaling/api/v1"
)

// TrafficStatReconciler reconciles a TrafficStat object
type TrafficStatReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=hybridscaling.knativescaling.dcn.ssu.ac.kr,resources=trafficstats,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hybridscaling.knativescaling.dcn.ssu.ac.kr,resources=trafficstats/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hybridscaling.knativescaling.dcn.ssu.ac.kr,resources=trafficstats/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TrafficStat object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *TrafficStatReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("reconciling foo custom resource")

	// Get the TrafficStat resource that trigger the reconciliation request
	var TrafficStatCRD = hybridscalingv1.TrafficStat{}
	if err := r.Get(ctx, req.NamespacedName, &TrafficStatCRD); err != nil {
		log.Error(err, "unable to fetch client")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Store Wanted/Target ServiceName from CRD in TargetServiceName variable
	CRDTargetServiceName := TrafficStatCRD.Spec.ServiceName

	// Initialize Knative Serving Go Client
	// Ref:https://stackoverflow.com/questions/66199455/list-service-in-go
	// This Testbed's MasterNode kubeconfig path = "/root/.kube/config"

	kubeconfig := flag.String("kubeconfig", "/root/.kube/config", "absolute path to the kubeconfig file")
	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Error(err, "unable to BuildConfigFromFlags using clientcmd")
	}

	serving, err := servingv1client.NewForConfig(config)
	if err != nil {
		log.Error(err, "unable to create Knative Serving Go Client")
	}

	//**Get Service with name == TrafficStatCRD.spec.servicename
	TargetService, err := serving.Services("default").Get(ctx, CRDTargetServiceName, metav1.GetOptions{})
	if err != nil {
		log.Info("TargetService name from CRD is:", CRDTargetServiceName)
		log.Error(err, "TargetService from CRD is not available in cluster")
	} else {
		log.Info("TargetService name from CRD is:", CRDTargetServiceName)
		log.Info("Found TargetService in cluster:", TargetService.Name)
	}

	//**Get Service's Concurrency-Resources ConfigMap (CR ConfigMap) with name == TrafficStatCRD.spec.servicename
	TargetConfigMap := &corev1.ConfigMap{}
	FetchConfigMapObjectKey := client.ObjectKey{
		Namespace: "default",
		Name:      CRDTargetServiceName,
	}

	if err := r.Get(ctx, FetchConfigMapObjectKey, TargetConfigMap); err != nil {
		log.Error(err, "unable to fetch ConfigMap corresponding to CRDTargetService")
	} else {
		log.Info("Fetch ConfigMap sucessful:", TargetConfigMap.Name)
	}

	//**Scaling Logic:
	// If wanted service and CR ConfigMap (get from above) avaialble - NOT null:
	// Calculate optimal concurrency and resources request configuration based on TrafficStatCRD.spec.ScalingInputTraffic and Service's CR ConfigMap
	// CR setting with minimum TotalResourceUsage = chosen CR
	minimumCR_TotalResourcesUsage := float64(100000000)
	ScalingInputTrafficFloat, err := strconv.ParseFloat(TrafficStatCRD.Spec.ScalingInputTraffic, 64)
	if err != nil {
		log.Error(err, err.Error())
	}
	var chosen_resourceLevel string
	var chosen_concurrency string

	for resourceLevel, concurrency := range TargetConfigMap.Data {
		ConcurrencyFloat, Cerr := strconv.ParseFloat(concurrency, 64)
		if Cerr != nil {
			log.Error(Cerr, Cerr.Error())
		}
		resourceLevelFloat, Rerr := strconv.ParseFloat(resourceLevel, 64)
		if Rerr != nil {
			log.Error(Rerr, Rerr.Error())
		}
		NumberOfPod := ScalingInputTrafficFloat / ConcurrencyFloat
		ThisCR_TotalResourcesUsage := NumberOfPod * resourceLevelFloat
		log.Info(resourceLevel, concurrency, NumberOfPod, ThisCR_TotalResourcesUsage)
		if ThisCR_TotalResourcesUsage < minimumCR_TotalResourcesUsage {
			minimumCR_TotalResourcesUsage = ThisCR_TotalResourcesUsage
			chosen_resourceLevel = resourceLevel + "m"
			chosen_concurrency = concurrency
		}
	}

	log.Info("Chosen CR settings for Hybrid scaling is", chosen_resourceLevel, chosen_concurrency)

	//// Define Configuration Yaml Object
	//// When a new Service is created, Knative assign for that Service an annotation "serving.knative.dev/creator" = The user that created the service.
	//// In this experiment, the root user at k8s master node created --> serving.knative.dev/creator = kubernetes-admin
	//// To be able to update a new Configuration for a Service, Knative-K8s requires 2 things:
	//// - serving.knative.dev/creator must be the same one find on "kubectl describe ksvc <service-name>"'s Annotation
	////  --> Add this to new Configuration's metadata/annotations
	//// - New Configuration ResourceVersion must be the same with the current one.
	////  ---> GetResourceVersion() of current service, set it to the new one.
	////  ---> This line below do that "configuration.SetResourceVersion(service.GetResourceVersion())"
	NewServiceConfiguration := &servingv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CRDTargetServiceName,
			Namespace: "default",
			Labels: map[string]string{
				"app": CRDTargetServiceName,
			},
			Annotations: map[string]string{
				"serving.knative.dev/creator": "kubernetes-admin",
			},
		},
		Spec: servingv1.ServiceSpec{
			ConfigurationSpec: servingv1.ConfigurationSpec{
				Template: servingv1.RevisionTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": CRDTargetServiceName,
						},
						Annotations: map[string]string{
							"autoscaling.knative.dev/target": chosen_concurrency,
						},
					},
					Spec: servingv1.RevisionSpec{
						PodSpec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  CRDTargetServiceName,
									Image: "vudinhdai2505/test-app:v5",
									Resources: corev1.ResourceRequirements{
										Requests: map[corev1.ResourceName]resource.Quantity{
											"cpu": resource.MustParse(chosen_resourceLevel),
										},
										Limits: map[corev1.ResourceName]resource.Quantity{
											"cpu": resource.MustParse(chosen_resourceLevel),
										},
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 5000,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	log.Info("Creating new Configuration for service ", CRDTargetServiceName, " with CR setting ", chosen_resourceLevel, chosen_concurrency)

	//// Set ResourceVersion of new Configuration to the current Service's ResourceVersion (Required for Update)
	NewServiceConfiguration.SetResourceVersion(TargetService.GetResourceVersion())

	//// Call KnativeServingClient to create new Service Revision by updating current service with new Configuration
	NewServiceRevision, err := serving.Services("default").Update(ctx, NewServiceConfiguration, metav1.UpdateOptions{})
	if err != nil {
		log.Error(err, err.Error())
	} else {
		log.Info("New Service Revision Created")
		log.Info(NewServiceRevision.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TrafficStatReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hybridscalingv1.TrafficStat{}).
		Complete(r)
}
