package main

import (
	"context"
	"fmt"
	"golang.org/x/xerrors"
	"io/ioutil"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"tiedpenguin.com/podmgr/loader"
	"tiedpenguin.com/podmgr/util"
	"time"
)

const (
	Bandwidth = "bandwidth"
	Loss      = "loss"
)

// The limits are stored outside the pod annotations because service-level limits only appear in service annotations
type podEntry struct {
	pod      v1.Pod
	limits   map[string]string
	programs map[string]string
}

type PodManager struct {
	podList []podEntry
}

func (pm *PodManager) Run(clientSet *kubernetes.Clientset) error {
	// First, look for service level limits and add corresponding pods
	services, err := clientSet.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, e := range services.Items {
		bandwidthAnnotation, hasBandwidth := e.Annotations[Bandwidth]
		lossAnnotation, hasLoss := e.Annotations[Loss]

		if !hasBandwidth && !hasLoss {
			continue
		}

		// We find the service's pods based on their selector
		selector := labels.SelectorFromSet(e.Spec.Selector)
		pods, err := clientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return err
		}

		for _, f := range pods.Items {
			tempLimits := make(map[string]string)

			if hasBandwidth {
				tempLimits[Bandwidth] = bandwidthAnnotation
			}

			if hasLoss {
				tempLimits[Loss] = lossAnnotation
			}

			err = pm.addPod(&podEntry{
				pod:      f,
				limits:   tempLimits,
				programs: make(map[string]string),
			}, false)
			if err != nil {
				return err
			}
		}
	}

	// Second, look for pod level limits and add them
	pods, err := clientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, e := range pods.Items {
		bandwidthAnnotation, hasBandwidth := e.Annotations[Bandwidth]
		lossAnnotation, hasLoss := e.Annotations[Loss]

		if !hasBandwidth && !hasLoss {
			continue
		}

		tempLimits := make(map[string]string)

		if hasBandwidth {
			tempLimits[Bandwidth] = bandwidthAnnotation
		}

		if hasLoss {
			tempLimits[Loss] = lossAnnotation
		}

		err = pm.addPod(&podEntry{
			pod:      e,
			limits:   tempLimits,
			programs: make(map[string]string),
		}, true)
		if err != nil {
			return err
		}
	}

	// Third, set up an event watcher
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

				bandwidthAnnotation, hasBandwidth := pod.Annotations[Bandwidth]
				lossAnnotation, hasLoss := pod.Annotations[Loss]

				if !hasBandwidth && !hasLoss {
					return
				}

				tempLimits := make(map[string]string)

				if hasBandwidth {
					tempLimits[Bandwidth] = bandwidthAnnotation
				}

				if hasLoss {
					tempLimits[Loss] = lossAnnotation
				}

				err = pm.addPod(&podEntry{
					pod:      *pod,
					limits:   tempLimits,
					programs: make(map[string]string),
				}, false)
				if err != nil {
					util.Die(err)
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

				if err := pm.removePod(*pod); err != nil {
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
			pm.cleanup()

			fmt.Println("Unloading ebpf program.")
			//if err := Unload(); err != nil {
			//	util.Die(err)
			//}

			fmt.Println("Shutting down.")
			os.Exit(0)
		}
	}

	return nil
}

// Override is used for pods as pod-level limits are stronger than service-level ones
func (pm *PodManager) addPod(entry *podEntry, override bool) error {
	if util.IsKubernetesNamespace(entry.pod.Namespace) {
		return nil
	}

	if override {
		// Override; remove already existing pod
		if err := pm.removePod(entry.pod); err != nil {
			return err
		}
	} else {
		// No override; only proceed for new pods
		for _, e := range pm.podList {
			if e.pod.UID == entry.pod.UID {
				return nil
			}
		}
	}

	if bandwidthAnnotation, hasBandwidth := entry.limits[Bandwidth]; hasBandwidth {
		paramList := strings.Split(bandwidthAnnotation, " ")

		if len(paramList) != 3 {
			return xerrors.New("Invalid bandwidth parameters.")
		}

		if paramList[2] != "ingress" && paramList[2] != "egress" {
			return xerrors.New("Invalid interface. Use ingress or egress")
		}

		params := "-DBANDWIDTH=" + paramList[0]

		switch paramList[1] {
		case "bps":
			// No additional zeros
		case "kbps":
			params += "000"
		case "mbps":
			params += "000000"
		}

		err := loader.Load(&entry.pod, "edt", paramList[2], params)
		if err != nil {
			return err
		}

		entry.programs["edt"] = paramList[2]
	}

	if lossAnnotation, hasLoss := entry.limits[Loss]; hasLoss {
		paramList := strings.Split(lossAnnotation, " ")

		if len(paramList) < 3 {
			return xerrors.New("Invalid bandwidth parameters.")
		}

		if paramList[1] != "ingress" && paramList[1] != "egress" {
			return xerrors.New("Invalid interface. Use ingress or egress")
		}

		var params string

		switch paramList[0] {
		case "uniform":
			params = "-DDISTRIBUTION=0"

			if !strings.HasSuffix(paramList[2], "%") {
				return xerrors.New("Invalid loss percentage.")
			}

			percentage := strings.TrimSuffix(paramList[2], "%")

			if value, err := strconv.Atoi(percentage); err != nil || value < 0 || value > 100 {
				return xerrors.New("Invalid loss percentage.")
			}

			params += " -DPERCENTAGE=" + percentage
		case "exponential":
			params = "-DDISTRIBUTION=1"
		default:
			return xerrors.New("Invalid distribution. Use uniform or exponential")
		}

		err := loader.Load(&entry.pod, Loss, paramList[1], params)
		if err != nil {
			return err
		}

		entry.programs[Loss] = paramList[1]
	}

	fmt.Println("\nAdded pod: " + entry.pod.Namespace + "/" + entry.pod.Name)
	pm.podList = append(pm.podList, *entry)

	return nil
}

func (pm *PodManager) removePod(pod v1.Pod) error {
	if util.IsKubernetesNamespace(pod.Namespace) {
		return nil
	}

	isPresentInList := false
	index := 0
	entry := podEntry{}

	for i, e := range pm.podList {
		if e.pod.UID == pod.UID {
			isPresentInList = true
			index = i
			entry = e
			break
		}
	}

	if !isPresentInList {
		return nil
	}

	fmt.Println("\nDeleted pod: " + pod.Namespace + "/" + pod.Name)
	for k, v := range entry.programs {
		loader.Unload(&entry.pod, k, v)
	}
	pm.podList = append(pm.podList[:index], pm.podList[index+1:]...)

	return nil
}

func (pm *PodManager) cleanup() {
	// Create a temporary list
	var tempList []v1.Pod
	for _, e := range pm.podList {
		tempList = append(tempList, e.pod)
	}

	for _, e := range tempList {
		_ = pm.removePod(e)
	}

	// Remove files from the build directory
	files, _ := ioutil.ReadDir("build/")
	for _, e := range files {
		if !e.IsDir() {
			_ = exec.Command("/bin/bash", "-c", fmt.Sprintf(loader.DeleteCommand, strings.TrimSuffix(e.Name(), ".o"))).Run()
		}
	}

}
