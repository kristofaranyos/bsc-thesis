package util

import (
	v1 "k8s.io/api/core/v1"
	"os/exec"
	"strings"
)

var KubernetesNamespaces = []string{"kube-system", "kube-public", "kube-node-lease"}

func IsKubernetesNamespace(namespace string) bool {
	for _, e := range KubernetesNamespaces {
		if e == namespace {
			return true
		}
	}

	return false
}

func GetCgroup(container v1.ContainerStatus) (string, error) {
	// Format: containerd://<id>
	arr := strings.Split(container.ContainerID, "/")
	if len(arr) < 3 {
		panic(container.ContainerID)
	}

	cgroup, err := exec.Command("/bin/bash", "-c", "sudo find /sys/fs/cgroup/ | grep "+arr[2]+" | head -n 1").Output()
	if err != nil {
		return "", err
	}

	return strings.Trim(string(cgroup), "\n"), nil
}
