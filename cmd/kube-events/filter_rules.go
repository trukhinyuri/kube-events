package main

import (
	"sync"

	"k8s.io/api/apps/v1"
	apiCore "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type DeployFilter struct {
	generationMu  sync.Mutex
	generationMap map[types.UID]int64
}

func NewDeployFilter() *DeployFilter {
	return &DeployFilter{
		generationMap: make(map[types.UID]int64),
	}
}

func (df *DeployFilter) compareAndSwapGeneration(uid types.UID, newGeneration int64) bool {
	df.generationMu.Lock()
	defer df.generationMu.Unlock()

	if df.generationMap[uid] < newGeneration {
		df.generationMap[uid] = newGeneration
		if newGeneration > 1 {
			return true
		}
	}
	return false
}

func (df *DeployFilter) Filter(event watch.Event) bool {
	deploy, ok := event.Object.(*v1.Deployment)
	if !ok {
		return false
	}
	if event.Type == watch.Modified {
		return df.compareAndSwapGeneration(deploy.UID, deploy.Generation)
	}
	return true
}

func ResourceQuotaFilter(event watch.Event) bool {
	rq, ok := event.Object.(*apiCore.ResourceQuota)
	if !ok {
		return false
	}
	if event.Type == watch.Modified {
		specLimitsMemory := rq.Spec.Hard[apiCore.ResourceLimitsMemory]
		specLimitsCPU := rq.Spec.Hard[apiCore.ResourceLimitsCPU]
		specRequestsMemory := rq.Spec.Hard[apiCore.ResourceRequestsMemory]
		specRequestsCPU := rq.Spec.Hard[apiCore.ResourceRequestsCPU]

		statusLimitsMemory := rq.Status.Hard[apiCore.ResourceLimitsMemory]
		statusLimitsCPU := rq.Status.Hard[apiCore.ResourceLimitsCPU]
		statusRequestsMemory := rq.Status.Hard[apiCore.ResourceRequestsMemory]
		statusRequestsCPU := rq.Status.Hard[apiCore.ResourceRequestsCPU]

		return specLimitsMemory.Cmp(statusLimitsMemory) != 0 ||
			specLimitsCPU.Cmp(statusLimitsCPU) != 0 ||
			specRequestsMemory.Cmp(statusRequestsMemory) != 0 ||
			specRequestsCPU.Cmp(statusRequestsCPU) != 0
	}
	return true
}

func EventsFilter(event watch.Event) bool {
	switch event.Type {
	case watch.Added, watch.Error:
		//pass
	default:
		return false
	}

	kubeEvent, ok := event.Object.(*apiCore.Event)
	if !ok {
		return false
	}

	switch kubeEvent.InvolvedObject.Kind {
	case "Pod", "PersistentVolumeClaim", "Node":
		//pass
	default:
		return false
	}

	//Allow events reasons only from whitelist with messages not in blacklist
	return eventsWhitelist.check(kubeEvent)
}

func PVCFilter(event watch.Event) bool {
	pv, ok := event.Object.(*apiCore.PersistentVolumeClaim)
	if !ok {
		return false
	}
	if event.Type == watch.Modified {
		if len(pv.Finalizers) == 0 {
			return false
		}
		if pv.DeletionTimestamp != nil {
			return false
		}
	}
	return true
}
