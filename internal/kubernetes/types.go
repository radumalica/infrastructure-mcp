// Package kubernetes implements read/diagnostic access to Kubernetes
// clusters described in the inventory. Unlike internal/docker (which
// shells `docker` over an existing SSH connection), this adapter talks
// directly to each cluster's API server via k8s.io/client-go, using the
// kubeconfig named in the inventory entry — there is no SSH hop and no
// shell command to inject into.
package kubernetes

// PodEntry summarizes one pod, as returned by ListPods.
type PodEntry struct {
	Name      string
	Namespace string
	Phase     string
	Ready     string // "N/M" containers ready
	Restarts  int32
	Node      string
	StartTime string
	Reason    string // non-empty only for pods stuck in a waiting/terminated state worth surfacing
}

// EventEntry summarizes one Kubernetes event.
type EventEntry struct {
	Type      string // "Normal" or "Warning"
	Reason    string
	Object    string // e.g. "Pod/my-app-6f9c"
	Message   string
	Count     int32
	FirstSeen string
	LastSeen  string
}

// NodeEntry summarizes one cluster node.
type NodeEntry struct {
	Name             string
	Ready            bool
	Roles            []string
	KubeletVersion   string
	OSImage          string
	CPUCapacity      string
	MemoryCapacity   string
	Unschedulable    bool
	KernelVersion    string
	ContainerRuntime string
	InternalIP       string
}

// ContainerStatus is a single container's status within a described pod.
type ContainerStatus struct {
	Name         string
	Ready        bool
	RestartCount int32
	Image        string
	State        string // "running", "waiting", "terminated"
	Reason       string // populated for waiting/terminated
}

// PodDescription is a structured stand-in for `kubectl describe pod`:
// the pod's spec/status highlights plus its recent events, rather than
// kubectl's full free-text dump.
type PodDescription struct {
	Name       string
	Namespace  string
	Phase      string
	Node       string
	PodIP      string
	StartTime  string
	Containers []ContainerStatus
	Events     []EventEntry
}
