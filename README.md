# A Demo for Kubernetes Device Plugin

```bash
# Create a kubernetes instance with kind
kind create cluster --image kindest/node:v1.20.7

# Build plugin
./download-deps.sh v1.20.7
docker build . -t k8s-device-plugin-demo:0.0.1

# Remove deprecation file (device plugin stub does not register device if this file exists)
docker exec -d kind-control-plane rm /var/lib/kubelet/device-plugins/DEPRECATION

# Load image to kind
kind load docker-image k8s-device-plugin-demo:0.0.1

# Run plugin
kubectl create -f daemonset.yaml

# Run a consumer pod
kubectl create -f consumer.yaml

# Cleanup
kind delete cluster
docker image rm k8s-device-plugin-demo:0.0.1
```
