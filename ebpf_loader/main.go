package main

import (
	"flag"
	"fmt"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/homedir"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var podManager PodManager

	// Compile and load ebpf programs
	fmt.Println("Compiling ebpf program.")
	if err := Compile(); err != nil {
		Die(err)
	}

	fmt.Println("Loading ebpf program.")
	if err := Load(); err != nil {
		Die(err)
	}

	// Initialize kubernetes client
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
		panic(err.Error())
	}

	// Set up an event watcher
	source := cache.NewListWatchFromClient(clientSet.CoreV1().RESTClient(), string(v1.ResourcePods), "", fields.Everything())
	_, k8sController := cache.NewInformer(
		source,
		&v1.Pod{},
		1*time.Minute,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod, ok := obj.(*v1.Pod)
				if !ok {
					return
				}

				if err := podManager.AddPod(pod); err != nil {
					fmt.Println(err)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				// We don't care about updating pods
			},
			DeleteFunc: func(obj interface{}) {
				pod, ok := obj.(*v1.Pod)
				if !ok {
					return
				}

				if err := podManager.RemovePod(pod); err != nil {
					fmt.Println(err)
				}
			},
		},
	)

	stop := make(chan struct{}, 1)
	go k8sController.Run(stop)

	interruptChannel := make(chan os.Signal)
	signal.Notify(interruptChannel, os.Interrupt, os.Kill)

	for {
		select {
		case <-time.After(time.Second):
			// Empty

		case <-interruptChannel:
			fmt.Println("") // To skip a line after "^C"
			close(stop)
			podManager.Cleanup()

			fmt.Println("Unloading ebpf program.")
			if err := Unload(); err != nil {
				Die(err)
			}

			fmt.Println("Shutting down.")
			os.Exit(0)
		}
	}
}