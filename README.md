# knative-hybrid-scaling

A Knative hybrid auto-scaling operator 

## Description
This operator take predicted traffic and pod's assigned resource level pairing with its corresponding optimal concurrency level as inputs

Inputs:
- Predicted Traffic is represented by TrafficStat Custom Resource produced by https://github.com/mipearlska/Predictive_TrafficStatCRD
- Resource-Optimal Concurrency pair of each service is represented by a ConfigMap

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
You’ll need a Knative cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/knative-hybrid-scaling:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/knative-hybrid-scaling:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

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

