package main

import (
	"context"
	"fmt"
	"golang.org/x/xerrors"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"os"
	"os/signal"
	"strings"
	"tiedpenguin.com/gotest/util"
	"time"
)

const (
	Bandwidth = "bandwidth"
	Loss      = "loss"
)

// The limits are stored outside the pod annotations because service-level limits only appear in service annotations
type podEntry struct {
	pod    v1.Pod
	limits map[string]string
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
				pod:    f,
				limits: tempLimits,
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
			pod:    e,
			limits: tempLimits,
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
					pod:    *pod,
					limits: tempLimits,
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

				if err := pm.removePod(pod); err != nil {
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
		if err := pm.removePod(&entry.pod); err != nil {
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

	fmt.Println("\nAdded pod: " + entry.pod.Namespace + "/" + entry.pod.Name)
	pm.podList = append(pm.podList, *entry)

	if bandwidthAnnotation, hasBandwidth := entry.limits[Bandwidth]; hasBandwidth {
		paramList := strings.Split(bandwidthAnnotation, " ")

		if len(paramList) != 3 {
			return xerrors.New("Invalid bandwidth parameters.")
		}

		params := "-DBANDWIDTH=" + paramList[0]

		switch paramList[1] {
		case "kbps":
			params += "000"
		case "mbps":
			params += "000000"
		}

		params += " -DINTERFACE=" + paramList[2]

		err := Load(&entry.pod, Bandwidth, params)
		if err != nil {
			return err
		}
	}

	if lossAnnotation, hasLoss := entry.limits[Loss]; hasLoss {
		// Compile program for it, etc
		fmt.Println(lossAnnotation)
	}

	return nil
}

func (pm *PodManager) removePod(pod *v1.Pod) error {
	if util.IsKubernetesNamespace(pod.Namespace) {
		return nil
	}

	isPresentInList := false
	index := 0

	for i, e := range pm.podList {
		if e.pod.UID == pod.UID {
			isPresentInList = true
			index = i
			break
		}
	}

	if !isPresentInList {
		return nil
	}

	fmt.Println("\nDeleted pod: " + pod.Namespace + "/" + pod.Name)
	pm.podList = append(pm.podList[:index], pm.podList[index+1:]...)

	return nil
}

func (pm *PodManager) cleanup() {
	// Create a temporary list
	var tempList []*v1.Pod
	for _, e := range pm.podList {
		tempList = append(tempList, &e.pod)
	}

	for _, e := range tempList {
		_ = pm.removePod(e)
	}
}
