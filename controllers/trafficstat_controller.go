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
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

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

//+kubebuilder:rbac:groups=serving.knative.dev,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

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
	} else {
		log.Info("Fetched TrafficStatCRD, target service is: ", "TARGET_SERVICE", TrafficStatCRD.Spec.ServiceName)
	}
	// Store Wanted/Target ServiceName from CRD in TargetServiceName variable
	CRDTargetServiceName := TrafficStatCRD.Spec.ServiceName

	// Initialize Knative Serving Go Client
	// Ref:https://stackoverflow.com/questions/66199455/list-service-in-go
	// This Testbed's MasterNode kubeconfig path = "/root/.kube/config"

	config, err := clientcmd.BuildConfigFromFlags("", "/root/.kube/config")
	if err != nil {
		log.Error(err, "unable to BuildConfigFromFlags using clientcmd")
	}

	serving, err := servingv1client.NewForConfig(config)
	if err != nil {
		log.Error(err, "unable to create Knative Serving Go Client")
	}

	//**Get Service's Concurrency-Resources ConfigMap (CR ConfigMap) with name == TrafficStatCRD.spec.servicename
	TargetConfigMap := &corev1.ConfigMap{}
	FetchConfigMapObjectKey := client.ObjectKey{
		Namespace: "default",
		Name:      "hybrid-" + CRDTargetServiceName,
	}

	if err := r.Get(ctx, FetchConfigMapObjectKey, TargetConfigMap); err != nil {
		log.Error(err, "unable to fetch ConfigMap corresponding to CRDTargetService")
	} else {
		log.Info("Fetch ConfigMap sucessful:", "CONFIG_MAP-NAME", TargetConfigMap.Name)
	}

	//**Get Service with name == TrafficStatCRD.spec.servicename
	TargetService, err := serving.Services("default").Get(ctx, CRDTargetServiceName, metav1.GetOptions{})
	if err != nil {
		log.Info("TargetService name from CRD is:", "SERVICE_NAME", CRDTargetServiceName)
		log.Error(err, "TargetService from CRD is not available in cluster")
	} else {
		log.Info("TargetService name from CRD is:", "SERVICE_NAME", CRDTargetServiceName)
		log.Info("Found TargetService in cluster:", "SERVICE_NAME", TargetService.Name)
	}

	TargetService_Type := TargetConfigMap.Data["resources-intensive-type"]
	TargetService_RequiredResources := TargetConfigMap.Data["required-resources"]
	TargetService_Current_Revision := TargetService.Status.LatestReadyRevisionName
	TargetService_Current_Pair_Concurrency := TargetService.Spec.Template.ObjectMeta.Annotations["autoscaling.knative.dev/target"]
	TargetService_Current_Pair_Resources_Limit := TargetService.Spec.Template.Spec.Containers[0].Resources.Limits
	TargetService_Current_Pair_Resources := ""

	if TargetService_Type == "cpu" {
		temp1 := strings.Split(fmt.Sprintf("%v", TargetService_Current_Pair_Resources_Limit["cpu"]), "=")
		temp2 := strings.Split(temp1[0], " ")
		TargetService_Current_Pair_Resources = temp2[0][2:] + "m"
	}
	if TargetService_Type == "memory" {
		temp1 := strings.Split(fmt.Sprintf("%v", TargetService_Current_Pair_Resources_Limit["memory"]), "=")
		temp2 := strings.Split(temp1[0], " ")
		temp3, err := strconv.Atoi(temp2[0][2:])
		if err != nil {
			log.Error(err, err.Error())
		}
		TargetService_Current_Pair_Resources = strconv.Itoa(temp3/1048576) + "Mi"
	}
	log.Info("TargetService Type is", "TYPE", TargetService_Type)
	log.Info("TargetService Required Resources is", "Required_RESOURCE", TargetService_RequiredResources)
	log.Info("TargetService Current Pair-Concurrency is", "Current_Pair_CONCURRENCY", TargetService_Current_Pair_Concurrency)
	log.Info("TargetService Current Pair-Resources is", "Current_Pair_RESOURCES", TargetService_Current_Pair_Resources)

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
	var chosen_numberofpod string

	for resourceLevel, concurrency := range TargetConfigMap.Data {
		if resourceLevel != "resources-intensive-type" && resourceLevel != "required-resources" {
			ConcurrencyFloat, Cerr := strconv.ParseFloat(concurrency, 64)
			if Cerr != nil {
				log.Error(Cerr, Cerr.Error())
			}
			resourceLevelFloat, Rerr := strconv.ParseFloat(resourceLevel, 64)
			if Rerr != nil {
				log.Error(Rerr, Rerr.Error())
			}
			NumberOfPod := math.Ceil(ScalingInputTrafficFloat / ConcurrencyFloat)
			ThisCR_TotalResourcesUsage := NumberOfPod * resourceLevelFloat
			log.Info("CR Pair", "CR_PAIR", resourceLevel+concurrency)
			log.Info("This CR Pair Expected NumberOfPod", "EX_NUMBER_OF_PODS", fmt.Sprintf("%v", NumberOfPod))
			log.Info("This CR Pair Expected Total Resources Usage", "EX_TOTAL_RESOURCES", fmt.Sprintf("%v", ThisCR_TotalResourcesUsage))
			if ThisCR_TotalResourcesUsage < minimumCR_TotalResourcesUsage {
				minimumCR_TotalResourcesUsage = ThisCR_TotalResourcesUsage
				if TargetService_Type == "cpu" {
					chosen_resourceLevel = resourceLevel + "m"
				} else if TargetService_Type == "memory" {
					chosen_resourceLevel = resourceLevel + "Mi"
				}
				chosen_concurrency = concurrency
				chosen_numberofpod = strconv.FormatFloat(NumberOfPod, 'g', 1, 64)
			}
		}
	}

	//// Only Update Service to a new Revision/Configuration if the new calculated autoscaling settings (res-con) is DIFFERENT with the current one
	if chosen_concurrency == TargetService_Current_Pair_Concurrency && chosen_resourceLevel == TargetService_Current_Pair_Resources {
		log.Info("Keep current service res-con autoscaling setting")
	} else {

		log.Info("Chosen CR settings for Hybrid scaling is", "RESOURCE", chosen_resourceLevel)
		log.Info("Chosen CR settings for Hybrid scaling is", "CONCURRENCY", chosen_concurrency)
		log.Info("Chosen CR settings for Hybrid scaling is", "NUMBEROFPOD", chosen_numberofpod)

		//// Define Configuration Yaml Object
		//// When a new Service is created, Knative assign for that Service an annotation "serving.knative.dev/creator" = The user that created the service.
		//// In this experiment, the root user at k8s master node created --> serving.knative.dev/creator = kubernetes-admin
		//// To be able to update a new Configuration for a Service, Knative-K8s requires 2 things:
		//// - serving.knative.dev/creator must be the same one find on "kubectl describe ksvc <service-name>"'s Annotation
		////  --> Add this to new Configuration's metadata/annotations
		//// - New Configuration ResourceVersion must be the same with the current one.
		////  ---> GetResourceVersion() of current service, set it to the new one.
		////  ---> This line below do that "configuration.SetResourceVersion(service.GetResourceVersion())"
		var NewServiceConfiguration *servingv1.Service

		if TargetService_Type == "cpu" {

			NewServiceConfiguration = &servingv1.Service{
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
									"autoscaling.knative.dev/target":        chosen_concurrency,
									"autoscaling.knative.dev/initial-scale": chosen_numberofpod,
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
													"memory": resource.MustParse(TargetService_RequiredResources),
												},
												Limits: map[corev1.ResourceName]resource.Quantity{
													"cpu":    resource.MustParse(chosen_resourceLevel),
													"memory": resource.MustParse(TargetService_RequiredResources),
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

		} else if TargetService_Type == "memory" {

			NewServiceConfiguration = &servingv1.Service{
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
									"autoscaling.knative.dev/target":        chosen_concurrency,
									"autoscaling.knative.dev/initial-scale": chosen_numberofpod,
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
													"cpu": resource.MustParse(TargetService_RequiredResources),
												},
												Limits: map[corev1.ResourceName]resource.Quantity{
													"cpu":    resource.MustParse(TargetService_RequiredResources),
													"memory": resource.MustParse(chosen_resourceLevel),
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
		}

		log.Info("Creating new Configuration for service ", "SERVICE_NAME", CRDTargetServiceName)
		log.Info("with chosen settings ", "CHOSEN_RESOURCE_LEVEL", chosen_resourceLevel)
		log.Info("with chosen settings ", "CHOSEN_RESOURCE_LEVEL", chosen_concurrency)

		//// Set ResourceVersion of new Configuration to the current Service's ResourceVersion (Required for Update)
		NewServiceConfiguration.SetResourceVersion(TargetService.GetResourceVersion())

		//// Call KnativeServingClient to create new Service Revision by updating current service with new Configuration
		NewServiceRevision, err := serving.Services("default").Update(ctx, NewServiceConfiguration, metav1.UpdateOptions{})

		// New Revision Number = current + 1 (from service-00009 to service-00010) (Below are string processing to get the new Revision ID/Number)
		tempstring := strings.Split(TargetService_Current_Revision, "-")
		tempint, _ := strconv.Atoi(tempstring[len(tempstring)-1])
		rev_number := strconv.Itoa(tempint + 1)
		New_Revision_Number := CRDTargetServiceName + "-" + strings.Repeat("0", 5-len(rev_number)) + rev_number
		if err != nil {
			log.Error(err, err.Error())
		} else {
			log.Info("New Service Revision Created", "SERVICE", NewServiceRevision.Name)
			log.Info("New Service Revision Number", "REV_NUMBER", New_Revision_Number)
		}

		//// Watch New Revision,
		//// Wait until new Revision ready (Pod Running)
		//// Delete old Revision and the corresponding pods (to handle previous Revision long Terminating pods time, which can hold a lot of worker node resources)

		// While Loop to wait until New Revision Pod Ready to serve
		for {
			BeforeDeleteRevisionPodList := &corev1.PodList{}
			if err := r.List(ctx, BeforeDeleteRevisionPodList); err != nil {
				log.Error(err, err.Error())
				break
			}
			count := 0
			newPodDeploy := false
			for _, pod := range BeforeDeleteRevisionPodList.Items {
				if strings.HasPrefix(pod.Name, New_Revision_Number) {
					newPodDeploy = true
				}
				if strings.HasPrefix(pod.Name, New_Revision_Number) && pod.Status.Phase != "Running" {
					newPodDeploy = false // New Pod Not Ready, keep previous Revision alive
					count += 1
				}
			}
			if count == 0 || !newPodDeploy {
				log.Info("New Revision Pod NOT READY")
			} else { // only when New Revision Pod Ready, process to Delete Previous Revision Pods step
				log.Info("New Revision Pod Running")
				break
			}
		}

		log.Info("Wait")
		time.Sleep(5 * time.Second)

		// New Revision Pods are READY now, Delete old Revision and old Revision pods
		// Check if old Revision Pods are still Terminating. If YES delete old Revision, Then Delete pod
		ReadyDeleteRevisionPodList := &corev1.PodList{}
		if err := r.List(ctx, ReadyDeleteRevisionPodList); err != nil {
			log.Error(err, err.Error())
		} else {
			count := 0 // count to ensure Delete Revision is only called one time in the PodList loop (when count = 1)
			for _, pod := range ReadyDeleteRevisionPodList.Items {
				if strings.HasPrefix(pod.Name, TargetService_Current_Revision) {
					targetpod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      pod.Name,
						},
					}
					count += 1
					if count == 1 {
						log.Info("Ask to delete Revision", "REVISION_NAME", TargetService_Current_Revision)

						err := serving.Revisions("default").Delete(context.Background(), TargetService_Current_Revision, metav1.DeleteOptions{})
						if err != nil {
							log.Error(err, err.Error())
						} else {
							log.Info("Delete Revision ", "REVISION_NAME", TargetService_Current_Revision)
						}
						time.Sleep(2 * time.Second)
					}

					if err := r.Delete(ctx, targetpod, client.GracePeriodSeconds(0)); err != nil {
						log.Error(err, err.Error())
					} else {
						log.Info("Delete pod ", "POD_NAME", pod.Name)
					}
				}
			}
		}
	}

	return ctrl.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *TrafficStatReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hybridscalingv1.TrafficStat{}).
		Complete(r)
}
