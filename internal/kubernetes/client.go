package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"infrastructure-mcp/internal/inventory"
)

// ClusterLookup resolves an inventory cluster name to its kubeconfig
// details. *inventory.Inventory satisfies this; tests substitute a fake.
type ClusterLookup interface {
	KubeCluster(name string) (inventory.KubeCluster, error)
}

// Client resolves inventory Kubernetes cluster names to a cached
// *kubernetes.Clientset per cluster, building each lazily from its
// kubeconfig on first use. Safe for concurrent use.
type Client struct {
	inv ClusterLookup

	mu        sync.Mutex
	clientset map[string]kubernetes.Interface
}

// New creates a Client that resolves cluster names against inv.
func New(inv ClusterLookup) *Client {
	return &Client{inv: inv, clientset: make(map[string]kubernetes.Interface)}
}

func (c *Client) clientFor(cluster string) (kubernetes.Interface, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cs, ok := c.clientset[cluster]; ok {
		return cs, nil
	}

	kc, err := c.inv.KubeCluster(cluster)
	if err != nil {
		return nil, err
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kc.Kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if kc.Context != "" {
		overrides.CurrentContext = kc.Context
	}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubernetes: build client config for cluster %q: %w", cluster, err)
	}

	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("kubernetes: build clientset for cluster %q: %w", cluster, err)
	}

	c.clientset[cluster] = cs
	return cs, nil
}

// ListPods returns pods in namespace (all namespaces if empty).
func (c *Client) ListPods(ctx context.Context, cluster, namespace string) ([]PodEntry, error) {
	cs, err := c.clientFor(cluster)
	if err != nil {
		return nil, err
	}

	list, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("kubernetes: list pods: %w", err)
	}

	entries := make([]PodEntry, 0, len(list.Items))
	for _, p := range list.Items {
		entries = append(entries, podEntry(p))
	}
	return entries, nil
}

// Logs returns up to tailLines of the given container's logs (the pod's
// only/first container if containerName is empty).
func (c *Client) Logs(ctx context.Context, cluster, namespace, pod, containerName string, tailLines int64) (string, error) {
	cs, err := c.clientFor(cluster)
	if err != nil {
		return "", err
	}

	opts := &corev1.PodLogOptions{Container: containerName}
	if tailLines > 0 {
		opts.TailLines = &tailLines
	}

	raw, err := cs.CoreV1().Pods(namespace).GetLogs(pod, opts).DoRaw(ctx)
	if err != nil {
		return "", fmt.Errorf("kubernetes: get logs: %w", err)
	}
	return string(raw), nil
}

// ListEvents returns events in namespace (all namespaces if empty), most
// recent first.
func (c *Client) ListEvents(ctx context.Context, cluster, namespace string) ([]EventEntry, error) {
	cs, err := c.clientFor(cluster)
	if err != nil {
		return nil, err
	}

	list, err := cs.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("kubernetes: list events: %w", err)
	}

	entries := make([]EventEntry, 0, len(list.Items))
	for _, e := range list.Items {
		entries = append(entries, eventEntry(e))
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].LastSeen > entries[j].LastSeen })
	return entries, nil
}

// DescribePod returns a structured summary of one pod plus its recent
// events — a stand-in for `kubectl describe pod`, not a byte-for-byte
// port of its free-text output.
func (c *Client) DescribePod(ctx context.Context, cluster, namespace, pod string) (PodDescription, error) {
	cs, err := c.clientFor(cluster)
	if err != nil {
		return PodDescription{}, err
	}

	p, err := cs.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return PodDescription{}, fmt.Errorf("kubernetes: get pod: %w", err)
	}

	events, err := cs.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", pod),
	})
	if err != nil {
		return PodDescription{}, fmt.Errorf("kubernetes: list pod events: %w", err)
	}

	eventEntries := make([]EventEntry, 0, len(events.Items))
	for _, e := range events.Items {
		eventEntries = append(eventEntries, eventEntry(e))
	}
	sort.Slice(eventEntries, func(i, j int) bool { return eventEntries[i].LastSeen > eventEntries[j].LastSeen })

	containers := make([]ContainerStatus, 0, len(p.Status.ContainerStatuses))
	for _, cst := range p.Status.ContainerStatuses {
		containers = append(containers, containerStatus(cst))
	}

	return PodDescription{
		Name:       p.Name,
		Namespace:  p.Namespace,
		Phase:      string(p.Status.Phase),
		Node:       p.Spec.NodeName,
		PodIP:      p.Status.PodIP,
		StartTime:  formatTime(p.Status.StartTime),
		Containers: containers,
		Events:     eventEntries,
	}, nil
}

// ListNodes returns every node in the cluster.
func (c *Client) ListNodes(ctx context.Context, cluster string) ([]NodeEntry, error) {
	cs, err := c.clientFor(cluster)
	if err != nil {
		return nil, err
	}

	list, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("kubernetes: list nodes: %w", err)
	}

	entries := make([]NodeEntry, 0, len(list.Items))
	for _, n := range list.Items {
		entries = append(entries, nodeEntry(n))
	}
	return entries, nil
}

func podEntry(p corev1.Pod) PodEntry {
	ready, total := 0, len(p.Status.ContainerStatuses)
	var restarts int32
	var reason string
	for _, cst := range p.Status.ContainerStatuses {
		if cst.Ready {
			ready++
		}
		restarts += cst.RestartCount
		if reason == "" {
			if cst.State.Waiting != nil {
				reason = cst.State.Waiting.Reason
			} else if cst.State.Terminated != nil && cst.State.Terminated.Reason != "" {
				reason = cst.State.Terminated.Reason
			}
		}
	}
	return PodEntry{
		Name:      p.Name,
		Namespace: p.Namespace,
		Phase:     string(p.Status.Phase),
		Ready:     fmt.Sprintf("%d/%d", ready, total),
		Restarts:  restarts,
		Node:      p.Spec.NodeName,
		StartTime: formatTime(p.Status.StartTime),
		Reason:    reason,
	}
}

func eventEntry(e corev1.Event) EventEntry {
	return EventEntry{
		Type:      e.Type,
		Reason:    e.Reason,
		Object:    fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
		Message:   e.Message,
		Count:     e.Count,
		FirstSeen: formatTime(&e.FirstTimestamp),
		LastSeen:  formatTime(&e.LastTimestamp),
	}
}

func containerStatus(cst corev1.ContainerStatus) ContainerStatus {
	state, reason := "running", ""
	switch {
	case cst.State.Waiting != nil:
		state = "waiting"
		reason = cst.State.Waiting.Reason
	case cst.State.Terminated != nil:
		state = "terminated"
		reason = cst.State.Terminated.Reason
	}
	return ContainerStatus{
		Name:         cst.Name,
		Ready:        cst.Ready,
		RestartCount: cst.RestartCount,
		Image:        cst.Image,
		State:        state,
		Reason:       reason,
	}
}

func nodeEntry(n corev1.Node) NodeEntry {
	ready := false
	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			ready = cond.Status == corev1.ConditionTrue
			break
		}
	}

	var roles []string
	for label := range n.Labels {
		if role, ok := strings.CutPrefix(label, "node-role.kubernetes.io/"); ok {
			roles = append(roles, role)
		}
	}
	sort.Strings(roles)

	var internalIP string
	for _, addr := range n.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			internalIP = addr.Address
			break
		}
	}

	cpu := n.Status.Capacity[corev1.ResourceCPU]
	mem := n.Status.Capacity[corev1.ResourceMemory]

	return NodeEntry{
		Name:             n.Name,
		Ready:            ready,
		Roles:            roles,
		KubeletVersion:   n.Status.NodeInfo.KubeletVersion,
		OSImage:          n.Status.NodeInfo.OSImage,
		CPUCapacity:      cpu.String(),
		MemoryCapacity:   mem.String(),
		Unschedulable:    n.Spec.Unschedulable,
		KernelVersion:    n.Status.NodeInfo.KernelVersion,
		ContainerRuntime: n.Status.NodeInfo.ContainerRuntimeVersion,
		InternalIP:       internalIP,
	}
}

func formatTime(t *metav1.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}
