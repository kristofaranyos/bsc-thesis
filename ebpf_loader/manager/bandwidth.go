package manager

import (
	"errors"
	"fmt"
	"k8s.io/api/core/v1"
	"os/exec"
	"tiedpenguin.com/gotest/util"
)

func AddLimit(pod *v1.Pod, bw string) error {
	for _, e := range pod.Status.ContainerStatuses {
		cgroup, err := util.GetCgroup(e)
		if err != nil {
			return err
		}

		if len(cgroup) == 0 {
			return errors.New("Empty cgroup for container: " + pod.Namespace + "/" + pod.Name + "/" + e.Name)
		}

		if err := exec.Command("/bin/bash", "-c", "sudo bpftool cgroup attach "+cgroup+"/ egress pinned /sys/fs/bpf/shaper/cgroup_skb_egress").Run(); err != nil {
			return errors.New("Couldn't attach ebpf program to: " + pod.Namespace + "/" + pod.Name + "/" + e.Name + ", error: " + err.Error())
		}

		fmt.Println("Applying bandwidth limit " + bw + " on container: " + pod.Namespace + "/" + pod.Name + "/" + e.Name)
	}

	return nil
}

func RemoveLimit(pod *v1.Pod, bw string) error {
	for _, e := range pod.Status.ContainerStatuses {
		cgroup, err := util.GetCgroup(e)
		if err != nil {
			return err
		}

		if len(cgroup) == 0 {
			return errors.New("Empty cgroup for container: " + pod.Namespace + "/" + pod.Name + "/" + e.Name)
		}

		if err := exec.Command("/bin/bash", "-c", "sudo bpftool cgroup detach "+cgroup+"/ egress pinned /sys/fs/bpf/shaper/cgroup_skb_egress").Run(); err != nil {
			return errors.New("Couldn't detach ebpf program to: " + pod.Namespace + "/" + pod.Name + "/" + e.Name + ", error: " + err.Error())
		}

		fmt.Println("Removing bandwidth limit " + bw + " from container: " + pod.Namespace + "/" + pod.Name + "/" + e.Name)
	}

	return nil
}
