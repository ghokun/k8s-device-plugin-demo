// I COPIED THIS FROM THE KUBELET SOURCE CODE
// AND MODIFIED SOME PARTS. ghokun

/*
Copyright 2018 The Kubernetes Authors.
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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	podresourcesv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
	"k8s.io/kubernetes/pkg/kubelet/apis/podresources"
	dm "k8s.io/kubernetes/pkg/kubelet/cm/devicemanager"
)

const (
	resourceName = "example.com/resource"

	grpcAddress    = "unix:///var/lib/kubelet/pod-resources/kubelet.sock"
	grpcBufferSize = 4 * 1024 * 1024
	grpcTimeout    = 5 * time.Second
	retryTimeout   = 1 * time.Second
	scrapeInterval = 10 * time.Second
)

var (
	devLabels = []string{"Dev_1", "Dev_2", "Dev_3", "Dev_4"}
	devs      = []*pluginapi.Device{
		{ID: "Dev_1", Health: pluginapi.Healthy},
		{ID: "Dev_2", Health: pluginapi.Healthy},
		{ID: "Dev_3", Health: pluginapi.Healthy},
		{ID: "Dev_4", Health: pluginapi.Healthy},
	}
	metrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pods_per_devices",
		Help: "Total number of pods per device",
	}, devLabels)
)

func getPodsUsingResource() {
	resListerClient, clientConn, err := podresources.GetV1Client(grpcAddress, grpcTimeout, grpcBufferSize)
	defer clientConn.Close()
	if err != nil {
		panic(err)
	}
	prometheus.MustRegister(metrics)

	for {
		resp, err := resListerClient.List(context.Background(), &podresourcesv1.ListPodResourcesRequest{})
		if err != nil {
			klog.Errorf("failed to list pod resources: %v", err)
		}
		if len(resp.PodResources) <= 0 {
			klog.Infof("No pods using resource %s", resourceName)
		}
		assignments := map[string][]string{}
		for _, dev := range devs {
			assignments[dev.ID] = []string{}
		}
		for _, podRes := range resp.PodResources { // for each pod
			for _, contRes := range podRes.Containers { // for each container
				for _, contDevices := range contRes.Devices { // for each device
					if contDevices.ResourceName == resourceName {
						for _, deviceId := range contDevices.DeviceIds { // for each device id
							assignments[deviceId] = append(assignments[deviceId], podRes.Name)
							// err = metrics.Write(&io_prometheus_client.Metric{Label: []*io_prometheus_client.LabelPair{{Name: &deviceId, Value: &podRes.Name}}})
							// if err != nil {
							// klog.Errorf("failed to write metrics: %v", err)
							// }
						}
					}
				}
			}
		}
		// for dev, podNames := range assignments {
		// klog.Infof("Device %s is assigned to pods %s", dev, strings.Join(podNames, ","))
		// }
		metrics.With(prometheus.Labels{
			"Dev_1": strings.Join(assignments["Dev_1"], ","),
			"Dev_2": strings.Join(assignments["Dev_2"], ","),
			"Dev_3": strings.Join(assignments["Dev_3"], ","),
			"Dev_4": strings.Join(assignments["Dev_4"], ","),
		}).Inc()
		time.Sleep(scrapeInterval)
	}
}

// stubAllocFunc creates and returns allocation response for the input allocate request
func stubAllocFunc(r *pluginapi.AllocateRequest, devs map[string]pluginapi.Device) (*pluginapi.AllocateResponse, error) {
	var responses pluginapi.AllocateResponse
	for _, req := range r.ContainerRequests {
		response := &pluginapi.ContainerAllocateResponse{}
		for _, requestID := range req.DevicesIDs {
			dev, ok := devs[requestID]
			if !ok {
				return nil, fmt.Errorf("invalid allocation request with non-existing device %s", requestID)
			}

			if dev.Health != pluginapi.Healthy {
				return nil, fmt.Errorf("invalid allocation request with unhealthy device: %s", requestID)
			}

			// create fake device file
			fpath := filepath.Join("/tmp", dev.ID)

			// clean first
			if err := os.RemoveAll(fpath); err != nil {
				return nil, fmt.Errorf("failed to clean fake device file from previous run: %s", err)
			}

			f, err := os.Create(fpath)
			if err != nil && !os.IsExist(err) {
				return nil, fmt.Errorf("failed to create fake device file: %s", err)
			}

			f.Close()

			response.Envs = map[string]string{}
			response.Envs["fpath"] = fpath

			response.Annotations = map[string]string{}
			response.Annotations["fpath"] = fpath

			response.Mounts = append(response.Mounts, &pluginapi.Mount{
				ContainerPath: fpath,
				HostPath:      fpath,
			})
		}
		responses.ContainerResponses = append(responses.ContainerResponses, response)
	}

	return &responses, nil
}

func main() {

	pluginSocksDir := pluginapi.DevicePluginPath //os.Getenv("PLUGIN_SOCK_DIR")
	klog.Infof("pluginSocksDir: %s", pluginSocksDir)
	if pluginSocksDir == "" {
		klog.Errorf("Empty pluginSocksDir")
		return
	}
	socketPath := pluginSocksDir + "dp." + fmt.Sprintf("%d", time.Now().Unix())

	dp1 := dm.NewDevicePluginStub(devs, socketPath, resourceName, false, false)
	if err := dp1.Start(); err != nil {
		panic(err)

	}
	dp1.SetAllocFunc(stubAllocFunc)
	if err := dp1.Register(pluginapi.KubeletSocket, resourceName, pluginapi.DevicePluginPath); err != nil {
		panic(err)
	}
	go getPodsUsingResource()
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
	select {}
}
