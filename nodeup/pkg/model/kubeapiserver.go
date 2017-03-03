/*
Copyright 2016 The Kubernetes Authors.

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

package model

import (
	"fmt"
	"strings"

	"k8s.io/kops/pkg/flagbuilder"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/nodeup/nodetasks"
	"k8s.io/apimachinery/pkg/api/resource"
	//k8sv1 "k8s.io/kubernetes/pkg/api/v1"
	k8sv1 "k8s.io/kubernetes/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// KubeAPIServerBuilder install kube-apiserver (just the manifest at the moment)
type KubeAPIServerBuilder struct {
	*NodeupModelContext
}

var _ fi.ModelBuilder = &KubeAPIServerBuilder{}

func (b *KubeAPIServerBuilder) Build(c *fi.ModelBuilderContext) error {
	if !b.IsMaster {
		return nil
	}

	{
		pod, err := b.buildPod()
		if err != nil {
			return fmt.Errorf("error building kube-apiserver manifest: %v", err)
		}

		manifest, err := ToVersionedYaml(pod)
		if err != nil {
			return fmt.Errorf("error marshalling manifest to yaml: %v", err)
		}

		t := &nodetasks.File{
			Path:     "/etc/kubernetes/manifests/kube-apiserver.manifest",
			Contents: fi.NewBytesResource(manifest),
			Type:     nodetasks.FileType_File,
		}
		c.AddTask(t)
	}

	return nil
}

func (b *KubeAPIServerBuilder) buildPod() (*k8sv1.Pod, error) {
	flags, err := flagbuilder.BuildFlags(b.Cluster.Spec.KubeAPIServer)
	if err != nil {
		return nil, fmt.Errorf("error building kube-apiserver flags: %v", err)
	}

	// Add cloud config file if needed
	if b.Cluster.Spec.CloudConfig != nil {
		flags += " --cloud-config=" + CloudConfigFilePath
	}

	redirectCommand := []string{
		"/bin/sh", "-c", "/usr/local/bin/kube-apiserver " + flags + " 1>>/var/log/kube-apiserver.log 2>&1",
	}

	pod := &k8sv1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "kube-apiserver",
			Namespace:   "kube-system",
			Annotations: b.buildAnnotations(),
			Labels: map[string]string{
				"k8s-app": "kube-apiserver",
			},
		},
		Spec: k8sv1.PodSpec{
			HostNetwork: true,
		},
	}

	container := &k8sv1.Container{
		Name:  "kube-apiserver",
		Image: b.Cluster.Spec.KubeAPIServer.Image,
		Resources: k8sv1.ResourceRequirements{
			Requests: k8sv1.ResourceList{
				k8sv1.ResourceCPU: resource.MustParse("150m"),
			},
		},
		Command: redirectCommand,
		LivenessProbe: &k8sv1.Probe{
			Handler: k8sv1.Handler{
				HTTPGet: &k8sv1.HTTPGetAction{
					Host: "127.0.0.1",
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      15,
		},
		Ports: []k8sv1.ContainerPort{
			{
				Name:          "https",
				ContainerPort: b.Cluster.Spec.KubeAPIServer.SecurePort,
				HostPort:      b.Cluster.Spec.KubeAPIServer.SecurePort,
			},
			{
				Name:          "local",
				ContainerPort: 8080,
				HostPort:      8080,
			},
		},
	}

	for _, path := range b.SSLHostPaths() {
		name := strings.Replace(path, "/", "", -1)

		addHostPathMapping(pod, container, name, path, true)
	}

	// Add cloud config file if needed
	if b.Cluster.Spec.CloudConfig != nil {
		addHostPathMapping(pod, container, "cloudconfig", CloudConfigFilePath, true)
	}

	if b.Cluster.Spec.KubeAPIServer.PathSrvKubernetes != "" {
		addHostPathMapping(pod, container, "srvkube", b.Cluster.Spec.KubeAPIServer.PathSrvKubernetes, true)
	}

	if b.Cluster.Spec.KubeAPIServer.PathSrvSshproxy != "" {
		addHostPathMapping(pod, container, "srvsshproxy", b.Cluster.Spec.KubeAPIServer.PathSrvSshproxy, false)
	}

	addHostPathMapping(pod, container, "logfile", "/var/log/kube-apiserver.log", false)

	pod.Spec.Containers = append(pod.Spec.Containers, *container)

	return pod, nil
}

func addHostPathMapping(pod *k8sv1.Pod, container *k8sv1.Container, name string, path string, readOnly bool) {
	pod.Spec.Volumes = append(pod.Spec.Volumes, k8sv1.Volume{
		Name: name,
		VolumeSource: k8sv1.VolumeSource{
			HostPath: &k8sv1.HostPathVolumeSource{
				Path: path,
			},
		},
	})

	container.VolumeMounts = append(container.VolumeMounts, k8sv1.VolumeMount{
		Name:      name,
		MountPath: path,
		ReadOnly:  readOnly,
	})
}

func (b *KubeAPIServerBuilder) buildAnnotations() map[string]string {
	annotations := make(map[string]string)
	annotations["dns.alpha.kubernetes.io/internal"] = b.Cluster.Spec.MasterInternalName
	if b.Cluster.Spec.API != nil && b.Cluster.Spec.API.DNS != nil {
		annotations["dns.alpha.kubernetes.io/external"] = b.Cluster.Spec.MasterPublicName
	}
	return annotations
}
