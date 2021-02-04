package main

import (
	"fmt"
	"os"
	"os/exec"
)

const (
	CompileCommand = "clang -target bpf -c -O2 bpf/shaper.c -o shaper.o -I/usr/include/x86_64-linux-gnu"
	LoadCommand    = "sudo bpftool prog loadall shaper.o /sys/fs/bpf/shaper type cgroup/skb"
	UnloadCommand  = "sudo rm /sys/fs/bpf/shaper/cgroup_skb_egress"
)

func Compile() error {
	return exec.Command("/bin/bash", "-c", CompileCommand).Run()
}

func Load() error {
	err := exec.Command("/bin/bash", "-c", LoadCommand).Run()
	if err != nil {
		// Try unloading the "old" one and retry
		_ = Unload()
		return exec.Command("/bin/bash", "-c", LoadCommand).Run()
	}

	return nil
}

func Unload() error {
	return exec.Command("/bin/bash", "-c", UnloadCommand).Run()
}

func Die(reason error) {
	fmt.Println(reason)
	os.Exit(-1)
}
