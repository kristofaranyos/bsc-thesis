package main

import (
	"fmt"
	"k8s.io/api/core/v1"
	"tiedpenguin.com/gotest/manager"
	"tiedpenguin.com/gotest/util"
)

type PodManager struct {
	podList []*v1.Pod
}

func (pm *PodManager) AddPod(pod *v1.Pod) error {
	if util.IsKubernetesNamespace(pod.Namespace) {
		return nil
	}

	isNewPod := true

	for _, e := range pm.podList {
		if e.UID == pod.UID {
			isNewPod = false
		}
	}

	if !isNewPod {
		return nil
	}

	fmt.Println("\nAdded pod: " + pod.Namespace + "/" + pod.Name)
	pm.podList = append(pm.podList, pod)

	if bandwidth, ok := pod.Annotations["bandwidth"]; ok {
		if err := manager.AddLimit(pod, bandwidth); err != nil {
			return err
		}
	}

	if loss, ok := pod.Annotations["loss"]; ok {
		if err := manager.Add(pod, loss); err != nil {
			return err
		}
	}

	return nil
}

func (pm *PodManager) RemovePod(pod *v1.Pod) error {
	if util.IsKubernetesNamespace(pod.Namespace) {
		return nil
	}

	isPresentInList := false
	index := 0

	for i, e := range pm.podList {
		if e.UID == pod.UID {
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

	if bandwidth, ok := pod.Annotations["bandwidth"]; ok {
		if err := manager.RemoveLimit(pod, bandwidth); err != nil {
			return err
		}
	}

	if loss, ok := pod.Annotations["loss"]; ok {
		if err := manager.Remove(pod, loss); err != nil {
			return err
		}
	}

	return nil
}

func (pm *PodManager) Cleanup() {
	var tempList []*v1.Pod
	for _, e := range pm.podList {
		tempList = append(tempList, e)
	}

	for _, e := range tempList {
		_ = pm.RemovePod(e)
	}
}
