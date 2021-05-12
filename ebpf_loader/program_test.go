package main

import (
	"io/ioutil"
	"os"
	"testing"
	"tiedpenguin.com/podmgr/loader"
)

func TestCompile(t *testing.T) {
	err := loader.CompileProgram("test", "test1", "")
	if err != nil {
		t.Error(err)
	}

	if _, err := os.Stat("build/test1.o"); os.IsNotExist(err) {
		t.Error("File doesn't exist")
	}
}

func TestDelete(t *testing.T) {
	err := ioutil.WriteFile("build/test2.o", []byte("test"), 0755)
	if err != nil {
		t.Error("Failed to make file")
	}

	_, err = loader.DeleteFile("test2")
	if err != nil {
		t.Error(err)
	}

	if _, err := os.Stat("build/test2.o"); !os.IsNotExist(err) {
		t.Error("File still exists")
	}
}

func TestLoad(t *testing.T) {
	err := loader.CompileProgram("bandwidth", "test3", "-DBANDWIDTH=100 -DINTERFACE='\"cgroup_skb/egress\"'")
	if err != nil {
		t.Error(err)
	}

	err = loader.LoadProgram("test3")
	if err != nil {
		t.Error(err)
	}

	if _, err := os.Stat("/sys/fs/bpf/test3"); os.IsNotExist(err) {
		t.Error("File doesn't exist")
	}
}

func TestUnload(t *testing.T) {
	err := loader.CompileProgram("bandwidth", "test4", "-DBANDWIDTH=100 -DINTERFACE='\"cgroup_skb/egress\"'")
	if err != nil {
		t.Error(err)
	}

	err = loader.LoadProgram("test4")
	if err != nil {
		t.Error(err)
	}

	_, err = loader.UnloadProgram("test4", "egress")
	if err != nil {
		t.Error(err)
	}

	if _, err := os.Stat("/sys/fs/bpf/test4"); !os.IsNotExist(err) {
		t.Error("File still exists")
	}
}
