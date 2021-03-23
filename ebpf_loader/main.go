package main

import (
	"flag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"tiedpenguin.com/gotest/util"
)

func main() {
	var podManager PodManager

	var configLocation *string
	if home := homedir.HomeDir(); home != "" {
		configLocation = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "Optional path to the kubeconfig file")
	} else {
		configLocation = flag.String("kubeconfig", "", "Path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *configLocation)
	if err != nil {
		panic(err.Error())
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		util.Die(err)
	}

	err = podManager.Run(clientSet)
	if err != nil {
		util.Die(err)
	}
}

//todo config change utan nem attacholja ujra
