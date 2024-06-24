# 0.Knative Install using Operator

- Required: ubuntu 18.04 (test on 18.04.5/6)
- Install Istio (via istioctl). Followed by install Knative Istio Controller
```
kubectl apply -f https://github.com/knative/net-istio/releases/download/knative-v1.8.0/net-istio.yaml
```
- Then, Follow official guides (install both serving + eventing)  (For Kubernetes 1.23.5, install version 1.8.5) (Knative >1.9 require K8s > 1.24)
- Then, config DNS
```
kubectl edit svc istio-ingressgateway -n istio-system
```
- add spec.externalIPs, change spec.type to NodePort
```
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
  creationTimestamp: "2023-04-13T09:44:30Z"
  labels:
    app: istio-ingressgateway
    install.operator.istio.io/owning-resource: unknown
    install.operator.istio.io/owning-resource-namespace: istio-system
    istio: ingressgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    operator.istio.io/managed: Reconcile
    operator.istio.io/version: 1.17.2
    release: istio
  name: istio-ingressgateway
  namespace: istio-system
  resourceVersion: "5537"
  uid: edfe40e4-adb6-4a2c-b661-c38242a1a3fb
spec:
  clusterIP: 10.100.244.70
  clusterIPs:
  - 10.100.244.70
  externalIPs:
  - 192.168.26.20
  externalTrafficPolicy: Cluster
  internalTrafficPolicy: Cluster
  ipFamilies:
  - IPv4
  ipFamilyPolicy: SingleStack
  ports:
  - name: status-port
    nodePort: 30913
    port: 15021
    protocol: TCP
    targetPort: 15021
  - name: http2
    nodePort: 31134
    port: 80
    protocol: TCP
    targetPort: 8080
  - name: https
    nodePort: 31326
    port: 443
    protocol: TCP
    targetPort: 8443
  selector:
    app: istio-ingressgateway
    istio: ingressgateway
  sessionAffinity: None
  type: NodePort
status:
  loadBalancer: {}
```
- Then:

kubectl patch configmap/config-domain \
      --namespace knative-serving \
      --type merge \
      --patch '{"data":{"192.168.26.20.nip.io":""}}

- Verify:
```
root@tgen:~# kubectl describe configmap config-domain -n knative-serving
Name:         config-domain
Namespace:    knative-serving
Labels:       app.kubernetes.io/component=controller
              app.kubernetes.io/name=knative-serving
              app.kubernetes.io/version=1.8.5
Annotations:  knative.dev/example-checksum: 26c09de5
              manifestival: new

Data
====
192.168.26.20.nip.io:
----

_example:
----
################################
#                              #
#    EXAMPLE CONFIGURATION     #
#                              #
################################

# This block is not actually functional configuration,
# but serves to illustrate the available configuration
# options and document them in a way that is accessible
# to users that `kubectl edit` this config map.
#
# These sample configuration options may be copied out of
# this example block and unindented to be in the data block
# to actually change the configuration.

# Default value for domain.
# Routes having the cluster domain suffix (by default 'svc.cluster.local')
# will not be exposed through Ingress. You can define your own label
# selector to assign that domain suffix to your Route here, or you can set
# the label
#    "networking.knative.dev/visibility=cluster-local"
# to achieve the same effect.  This shows how to make routes having
# the label app=secret only exposed to the local cluster.
svc.cluster.local: |
  selector:
    app: secret

# These are example settings of domain.
# example.com will be used for all routes, but it is the least-specific rule so it
# will only be used if no other domain matches.
example.com: |

# example.org will be used for routes having app=nonprofit.
example.org: |
  selector:
    app: nonprofit


BinaryData
====

Events:  <none>
```

watch -n 1 kubectl  get pod -o wide -A
```
kubectl --namespace istio-system get service istio-ingressgateway
```
```
NAME                   TYPE       CLUSTER-IP       EXTERNAL-IP     PORT(S)                                      AGE
istio-ingressgateway   NodePort   10.111.109.191   192.168.26.20   15021:31040/TCP,80:32586/TCP,443:31989/TCP   8m23s
```
```
curl -H "Host: helloworld-go.default.192.168.26.42.nip.io" http://192.168.26.20:32586
```

# 1. Install Prometheus Operator using Helm

#You will need to ensure that the helm chart has following values configured, otherwise the ServiceMonitors/Podmonitors will not work.

```
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm pull prometheus-community/kube-prometheus-stack  --untar

cd kube-prometheus-stack
```
- Edit following params in values.yaml
```
kube-state-metrics:
  metricLabelsAllowlist:
    - pods=[*]
    - deployments=[app.kubernetes.io/name,app.kubernetes.io/component,app.kubernetes.io/instance]
prometheus:
  prometheusSpec:
    serviceMonitorSelectorNilUsesHelmValues: false
    podMonitorSelectorNilUsesHelmValues: false
```
- Then install
```
helm install prometheus prometheus-community/kube-prometheus-stack -n default -f values.yaml
```

# 2. Apply the ServiceMonitors/PodMonitors to collect metrics from Knative.
```
kubectl apply -f https://raw.githubusercontent.com/knative-sandbox/monitoring/main/servicemonitor.yaml
```

# 3. Install Grafana using Helm

#Deploy Grafana Helm Chart with the dashboard sidecar enabled
```
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
helm pull grafana/grafana  --untar

cd grafana
```
- Edit following params in values.yaml
```
sidecar:
  dashboards:
    enabled: true
    searchNamespace: ALL
```

helm install grafana grafana/grafana -f values.yaml

# 4. If you are using the Grafana Helm Chart with the Dashboard Sidecar enabled, you can load the dashboards by applying the following configmaps.
```
kubectl apply -f https://raw.githubusercontent.com/knative-sandbox/monitoring/main/grafana/dashboards.yaml
```

# 5 Expose Prometheus, Grafana
```
kubectl expose service prometheus-operated --type=NodePort --target-port=9090 --name=prometheus-server
kubectl expose service grafana --type=NodePort --target-port=3000 --name=grafana-server
```
- Get Grafana password of admin
```
kubectl get secret --namespace default grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
```
```
root@ubuntu:~# kubectl get service
NAME                                      TYPE           CLUSTER-IP       EXTERNAL-IP                                            PORT(S)                                              AGE
alertmanager-operated                     ClusterIP      None             <none>                                                 9093/TCP,9094/TCP,9094/UDP                           46h
grafana                                   ClusterIP      10.97.196.43     <none>                                                 80/TCP                                               45h
grafana-server                            NodePort       10.102.82.24     <none>                                                 80:31827/TCP                                         29m
helloworld-go                             ExternalName   <none>           knative-local-gateway.istio-system.svc.cluster.local   80/TCP                                               34d
helloworld-go-00001                       ClusterIP      10.104.74.63     <none>                                                 80/TCP,443/TCP                                       34d
helloworld-go-00001-private               ClusterIP      10.103.199.75    <none>                                                 80/TCP,443/TCP,9090/TCP,9091/TCP,8022/TCP,8012/TCP   34d
kubernetes                                ClusterIP      10.96.0.1        <none>                                                 443/TCP                                              34d
operator-webhook                          ClusterIP      10.98.186.96     <none>                                                 9090/TCP,8008/TCP,443/TCP                            34d
prometheus-grafana                        ClusterIP      10.97.146.130    <none>                                                 80/TCP                                               46h
prometheus-kube-prometheus-alertmanager   ClusterIP      10.108.118.42    <none>                                                 9093/TCP                                             46h
prometheus-kube-prometheus-operator       ClusterIP      10.103.50.94     <none>                                                 443/TCP                                              46h
prometheus-kube-prometheus-prometheus     ClusterIP      10.101.75.198    <none>                                                 9090/TCP                                             46h
prometheus-kube-state-metrics             ClusterIP      10.100.30.190    <none>                                                 8080/TCP                                             46h
prometheus-operated                       ClusterIP      None             <none>                                                 9090/TCP                                             46h
prometheus-prometheus-node-exporter       ClusterIP      10.106.44.84     <none>                                                 9100/TCP                                             46h
prometheus-server                         NodePort       10.109.194.178   <none>                                                 9090:30693/TCP                                       32m
```

kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].port}' 


curl -H "Host: helloworld-go.default.example.com" http://192.168.40.67:30929
curl -H "Host: deploy-a.default.example.com" http://192.168.40.67:30929


# Locust traffic profile, add header if not config DNS

```
class QuickstartUser(HttpUser):
    wait_time = constant(1)

    @task(1)
    def index_page(self):
        self.client.get("/test", headers={"Host": "deploy-a.default.example.com"})
```
# kubebuilder
```
kubebuilder init --domain knative.dcn.ssu.ac.kr --owner mipearlska --project-name knative-hybrid-scaling --repo "github.com/mipearlska/knative_hybrid_scaling"

kubebuilder create api --controller true --group hybridscaling --version v1 --kind TrafficStat  --resource true
```
# Scaler application env version
```
absl-py==0.15.0
asn1crypto==0.24.0
astunparse==1.6.3
cached-property==1.5.2
cachetools==4.2.4
certifi==2021.10.8
charset-normalizer==2.0.12
clang==5.0
cryptography==2.1.4
cycler==0.11.0
dataclasses==0.8
flatbuffers==1.12
gast==0.4.0
google-auth==1.35.0
google-auth-oauthlib==0.4.6
google-pasta==0.2.0
grpcio==1.44.0
h5py==3.1.0
idna==2.6
importlib-metadata==4.8.3
joblib==1.1.0
keras==2.6.0
Keras-Preprocessing==1.1.2
keyring==10.6.0
keyrings.alt==3.0
kiwisolver==1.3.1
Markdown==3.3.6
matplotlib==3.3.4
numpy==1.19.5
oauthlib==3.2.0
opt-einsum==3.3.0
pandas==1.1.5
Pillow==8.4.0
protobuf==3.19.4
pyasn1==0.4.8
pyasn1-modules==0.2.8
pycrypto==2.6.1
PyGObject==3.26.1
pyparsing==3.0.8
python-dateutil==2.8.2
pytz==2022.1
pyxdg==0.25
requests==2.27.1
requests-oauthlib==1.3.1
rsa==4.8
schedule==1.1.0
scikit-learn==0.24.2
scipy==1.5.4
SecretStorage==2.3.1
six==1.15.0
sklearn==0.0
tensorboard==2.6.0
tensorboard-data-server==0.6.1
tensorboard-plugin-wit==1.8.1
tensorflow==2.6.2
tensorflow-estimator==2.6.0
termcolor==1.1.0
threadpoolctl==3.1.0
typing-extensions==3.7.4.3
urllib3==1.26.9
Werkzeug==2.0.3
wrapt==1.12.1
zipp==3.6.0
```

# service deploy-a.yaml
```
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: deploy-a
  namespace: default
  labels:
    app: deploy-a
spec:
  template:
    metadata:
      labels:
        app: deploy-a
    spec:
      containers:
      - name: deploy-a
        image: vudinhdai2505/test-app:v5
        resources:
          limits:
            cpu: "700m"
        ports:
        - containerPort: 5000
```
