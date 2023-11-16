# knative-hybrid-scaling

A Knative hybrid auto-scaling operator used in
- https://doi.org/10.1016/j.future.2023.11.010
***Optimized resource usage with hybrid auto-scaling system for knative serverless edge computing (Future Generation Computer Systems, Volume 152, March 2024, Pages 304-316)

## Description
- This operator take predicted traffic and pod's Knative hybrid autoscaling profile as inputs
  
Inputs:
- Predicted Traffic is represented by TrafficStat Custom Resource produced by https://github.com/mipearlska/Predictive_TrafficStatCRD
- Knative Hybrid AutoScaling profile: Resource-Optimal Concurrency pair of each service is represented by a ConfigMap
**Note:** Optimal Concurrency of a Resource level: Given the resource level, it is the maximum concurrency requests that a pod can handle while still guarantee service latency SLO

Example ConfigMap
```
apiVersion: v1
kind: ConfigMap
metadata:
  name: service-a
data:
  resources-intensive-type: "cpu"
  required-resources: "200Mi"
  "cpu1": "concurrency1"  
  "cpu2": "concurrency2"
  "cpu3": "concurrency3"  
  "cpu4": "concurrency4"
```

Example TrafficStat Custom Resource
```
apiVersion: hybridscaling.knative.dcn.ssu/v1
kind: TrafficStat
metadata:
  name: service-a-traffictest
spec:
  servicename: service-a
  predictedinputtraffic: "100"
```
## Getting Started
You’ll need a Knative cluster to run against.
**Note:** This operator was built and tested on: (Recommended for testing)
- Ubuntu version 18.04.5/6
- Kubernetes v1.23.5
- Istio
- Knative v1.8 (Serving and Eventing v1.8.5, Knative Istio Controller v1.8.0)
For installing the above environment, please refer to knative_install.md file.


### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### For Testing
0. Kubectl apply the target service as given in running_prequisites/deploy-testservice
1. Kubectl apply the target service's hybrid autoscaling profile as given in running_prequisites/testconfigmap
2. Delete any TrafficStat CR in cluster if running the test again from the beginning
3. Run Locust Traffic Profile as given in running_prequisites/locustservicetraffic (Not generate traffic yet)
4. Build and Install the CRDs into the cluster (Only first time)

```sh
make                              #make all
make manifests                    #Generate CRD
kubectl apply -f config/crd/bases #Apply CRD
```

5. Run HybridScaling controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

6. Start Generating Traffic to Service URL using Locust UI (HostURL:8089)
**Note:** Traffic need to be predicted before the actual new traffic change happening
- For example: Predict Traffic at every second 55 in a minute ("schedule.every().minute.at(":55").do(lambda: predict(api)")
- Then Start Generating Traffic at second >= 00 (example predicting at 0:55, new traffic change at 1:00)

7. Start Traffic Prediction Service (refer to https://github.com/mipearlska/Predictive_TrafficStatCRD for installation and running guide)
- This service might take up until 1 minute to start up (loading libraries, etc.)
- Recommend Running flow: Start Prediction service (running python3 main.py) at 0:40, Start Traffic Generation at 1:00
- First Prediction will happen at 1:55, at 0:55 the Prediction service is still starting up
- If your system startup the Prediction service fast, delay the Start Prediction service time to a later second (<55)
- Reason: Prediction Service up before 0:55, first prediction happen at 0:55 while the traffic has not been generated yet (scheduled to start at 1:00)

  
## License

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

