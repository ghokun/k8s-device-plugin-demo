#!/usr/bin/env bash

kubectl delete -f daemonset.yaml -f consumer.yaml

docker build . -t k8s-device-plugin-demo:0.0.1

kind load docker-image k8s-device-plugin-demo:0.0.1

kubectl create -f daemonset.yaml

kubectl create -f consumer.yaml
