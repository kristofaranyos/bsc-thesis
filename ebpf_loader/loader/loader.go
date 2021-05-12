package loader

import (
	"errors"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"os/exec"
	"strings"
	"tiedpenguin.com/podmgr/util"
)

const (
	CompileCommand = "clang -target bpf -c -O2 bpf/%s.c -o build/%s.o %s -I/usr/include/x86_64-linux-gnu"
	DeleteCommand  = "rm build/%s.o"
	LoadCommand    = "sudo bpftool prog loadall build/%[1]s.o /sys/fs/bpf/%[1]s type cgroup/skb"
	UnloadCommand  = "sudo rm /sys/fs/bpf/%s/cgroup_skb_%s"
	AttachCommand  = "sudo bpftool cgroup attach %s/ %s pinned /sys/fs/bpf/%s/cgroup_skb_%[2]s"
	DetachCommand  = "sudo bpftool cgroup detach %s/ %s pinned /sys/fs/bpf/%s/cgroup_skb_%[2]s"
)

func Load(pod *v1.Pod, programName, ioInterface, params string) error {
	fileName := programName + "_" + string(pod.UID)

	// Unload remnant if any
	if unloadOutput, err := UnloadProgram(fileName, ioInterface); err != nil && !strings.Contains(string(unloadOutput), "No such file or directory") {
		return err
	}

	// Remove remnant if any
	if cleanOutput, err := DeleteFile(fileName); err != nil && !strings.Contains(cleanOutput, "No such file or directory") {
		return err
	}

	// Compile ebpf program
	if err := CompileProgram(programName, fileName, params); err != nil {
		return err
	}

	// Load ebpf program
	if err := LoadProgram(fileName); err != nil {
		return err
	}

	// Attach to cgroup of processes
	for _, e := range pod.Status.ContainerStatuses {
		cgroup, err := util.GetCgroup(e)
		if err != nil {
			return err
		}

		if len(cgroup) == 0 {
			return errors.New("Empty cgroup for container: " + pod.Namespace + "/" + pod.Name + "/" + e.Name)
		}

		fmt.Println(fmt.Sprintf(AttachCommand, cgroup, ioInterface, fileName))
		if err := exec.Command("/bin/bash", "-c", fmt.Sprintf(AttachCommand, cgroup, ioInterface, fileName)).Run(); err != nil {
			return errors.New("Couldn't attach ebpf program to: " + pod.Namespace + "/" + pod.Name + "/" + e.Name + ", error: " + err.Error())
		}
	}

	return nil
}

func Unload(pod *v1.Pod, programName, ioInterface string) {
	fileName := programName + "_" + string(pod.UID)

	// Detach from cgroup of processes
	for _, e := range pod.Status.ContainerStatuses {
		cgroup, err := util.GetCgroup(e)
		if err == nil && len(cgroup) > 0 {
			_ = exec.Command("/bin/bash", "-c", fmt.Sprintf(DetachCommand, cgroup, ioInterface, fileName)).Run()
		}
	}

	// Unload ebpf program
	_, _ = UnloadProgram(fileName, ioInterface)

	// Delete program
	_, _ = DeleteFile(fileName)
}

func CompileProgram(programName, fileName, params string) error {
	fmt.Println(fmt.Sprintf(CompileCommand, programName, fileName, params))
	return exec.Command("/bin/bash", "-c", fmt.Sprintf(CompileCommand, programName, fileName, params)).Run()
}

func LoadProgram(fileName string) error {
	fmt.Println(fmt.Sprintf(LoadCommand, fileName))
	return exec.Command("/bin/bash", "-c", fmt.Sprintf(LoadCommand, fileName)).Run()
}

func UnloadProgram(fileName, ioInterface string) (string, error) {
	out, err := exec.Command("/bin/bash", "-c", fmt.Sprintf(UnloadCommand, fileName, ioInterface)).CombinedOutput()
	return string(out), err
}

func DeleteFile(fileName string) (string, error) {
	out, err := exec.Command("/bin/bash", "-c", fmt.Sprintf(DeleteCommand, fileName)).CombinedOutput()
	return string(out), err
}
