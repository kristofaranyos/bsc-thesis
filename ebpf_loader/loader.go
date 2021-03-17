package main

import (
	"errors"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"os/exec"
	"strings"
	"tiedpenguin.com/gotest/util"
)

const (
	CompileCommand = "clang -target bpf -c -O2 bpf/%s.c -o build/%s.o %s -I/usr/include/x86_64-linux-gnu"
	DeleteCommand  = "rm build/%s.o"
	LoadCommand    = "sudo bpftool prog loadall build/%[1]s.o /sys/fs/bpf/%[1]s type cgroup/skb"
	UnloadCommand  = "sudo rm /sys/fs/bpf/%s/cgroup_skb_egress"
)

func Load(pod *v1.Pod, programName, params string) error {
	fileName := programName + "_" + string(pod.UID)

	// Unload remnant if any
	unloadOutput, err := exec.Command("/bin/bash", "-c", UnloadCommand).CombinedOutput()
	if err != nil && !strings.Contains(string(unloadOutput), "No such file or directory") {
		return err
	}

	// Remove remnant if any
	cleanOutput, err := exec.Command("/bin/bash", "-c", fmt.Sprintf(DeleteCommand, fileName)).CombinedOutput()
	if err != nil && !strings.Contains(string(cleanOutput), "No such file or directory") {
		return err
	}

	// Compile ebpf program
	fmt.Println(fmt.Sprintf(CompileCommand, programName, fileName, params))
	msg, err := exec.Command("/bin/bash", "-c", fmt.Sprintf(CompileCommand, programName, fileName, params)).CombinedOutput()
	if err != nil {
		fmt.Println(string(msg))
		return err
	}

	// Load ebpf program
	err = exec.Command("/bin/bash", "-c", fmt.Sprintf(LoadCommand, fileName)).Run()
	if err != nil {
		return err
	}

	for _, e := range pod.Status.ContainerStatuses {
		cgroup, err := util.GetCgroup(e)
		if err != nil {
			return err
		}

		if len(cgroup) == 0 {
			return errors.New("Empty cgroup for container: " + pod.Namespace + "/" + pod.Name + "/" + e.Name)
		}

		if err := exec.Command("/bin/bash", "-c", "sudo bpftool cgroup attach "+cgroup+"/ egress pinned /sys/fs/bpf/"+fileName+"/cgroup_skb_egress").Run(); err != nil {
			return errors.New("Couldn't attach ebpf program to: " + pod.Namespace + "/" + pod.Name + "/" + e.Name + ", error: " + err.Error())
		}
	}

	return nil
}

func Unload(pod *v1.Pod, programName string) error {
	fileName := programName + "_" + string(pod.UID)

	// Unload ebpf program
	return exec.Command("/bin/bash", "-c", fmt.Sprintf(UnloadCommand, fileName)).Run()
}
