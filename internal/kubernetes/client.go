package kubernetes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	neturl "net/url"
	"sort"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	"infrastructure-mcp/internal/inventory"
)

// ClusterLookup resolves an inventory cluster name to its kubeconfig
// details. *inventory.Inventory satisfies this; tests substitute a fake.
type ClusterLookup interface {
	KubeCluster(name string) (inventory.KubeCluster, error)
}

// clusterConn caches everything Exec needs alongside the clientset: the
// clientset alone is enough for every other method, but Exec has to build
// its own SPDY-upgraded connection and therefore needs the raw *rest.Config
// too.
type clusterConn struct {
	clientset  kubernetes.Interface
	restConfig *rest.Config
}

// Client resolves inventory Kubernetes cluster names to a cached
// *kubernetes.Clientset per cluster, building each lazily from its
// kubeconfig on first use. Safe for concurrent use.
type Client struct {
	inv ClusterLookup

	mu    sync.Mutex
	conns map[string]clusterConn

	// stream performs the exec sub-resource's SPDY stream. A field
	// (rather than a hardcoded call to remotecommand.NewSPDYExecutor) so
	// tests can substitute a fake — the fake kubernetes.Interface used
	// everywhere else in this package has no real exec sub-resource to
	// hit, since it's a raw HTTP connection upgrade, not a REST verb a
	// reactor can intercept.
	stream streamFunc
}

// New creates a Client that resolves cluster names against inv.
func New(inv ClusterLookup) *Client {
	return &Client{inv: inv, conns: make(map[string]clusterConn), stream: defaultStream}
}

func (c *Client) clientFor(cluster string) (kubernetes.Interface, error) {
	conn, err := c.connFor(cluster)
	if err != nil {
		return nil, err
	}
	return conn.clientset, nil
}

func (c *Client) connFor(cluster string) (clusterConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if conn, ok := c.conns[cluster]; ok {
		return conn, nil
	}

	kc, err := c.inv.KubeCluster(cluster)
	if err != nil {
		return clusterConn{}, err
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kc.Kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if kc.Context != "" {
		overrides.CurrentContext = kc.Context
	}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return clusterConn{}, fmt.Errorf("kubernetes: build client config for cluster %q: %w", cluster, err)
	}

	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return clusterConn{}, fmt.Errorf("kubernetes: build clientset for cluster %q: %w", cluster, err)
	}

	conn := clusterConn{clientset: cs, restConfig: restConfig}
	c.conns[cluster] = conn
	return conn, nil
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

// streamFunc performs one exec sub-resource SPDY stream and returns the
// captured stdout/stderr and the command's exit code (0 if it completed
// without a non-zero-exit error).
type streamFunc func(cfg *rest.Config, method string, url *neturl.URL, stdin io.Reader) (stdout, stderr string, exitCode int, err error)

// defaultStream is streamFunc's real implementation, used outside tests.
func defaultStream(cfg *rest.Config, method string, url *neturl.URL, stdin io.Reader) (string, string, int, error) {
	executor, err := remotecommand.NewSPDYExecutor(cfg, method, url)
	if err != nil {
		return "", "", 0, err
	}

	var stdout, stderr bytes.Buffer
	err = executor.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		var exitErr interface{ ExitStatus() int }
		if errors.As(err, &exitErr) {
			return stdout.String(), stderr.String(), exitErr.ExitStatus(), nil
		}
		return "", "", 0, err
	}
	return stdout.String(), stderr.String(), 0, nil
}

// Exec runs command inside containerName of pod (the pod's only/first
// container if containerName is empty) via the exec sub-resource — the
// pod-scoped equivalent of internal/docker.Client.Exec. Like that method,
// and like the top-level run_command tool it mirrors, a non-zero exit
// code is reported as data (ExecResult.ExitCode), not a Go error.
func (c *Client) Exec(ctx context.Context, cluster, namespace, pod, containerName string, command []string) (ExecResult, error) {
	conn, err := c.connFor(cluster)
	if err != nil {
		return ExecResult{}, err
	}

	// Built directly via rest.RESTClientFor rather than
	// conn.clientset.CoreV1().RESTClient(): the fake clientset used in
	// tests returns a nil RESTClient (it has no real transport), so
	// Exec's own request-building has to stand on its own, the same way
	// production code would build it. Mirrors typed corev1 client's own
	// setConfigDefaults (GroupVersion/APIPath/NegotiatedSerializer).
	execConfig := rest.CopyConfig(conn.restConfig)
	gv := corev1.SchemeGroupVersion
	execConfig.GroupVersion = &gv
	execConfig.APIPath = "/api"
	execConfig.NegotiatedSerializer = rest.CodecFactoryForGeneratedClient(scheme.Scheme, scheme.Codecs).WithoutConversion()

	restClient, err := rest.RESTClientFor(execConfig)
	if err != nil {
		return ExecResult{}, fmt.Errorf("kubernetes: build exec client for cluster %q: %w", cluster, err)
	}

	req := restClient.Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	stdout, stderr, exitCode, err := c.stream(execConfig, "POST", req.URL(), nil)
	if err != nil {
		return ExecResult{}, fmt.Errorf("kubernetes: exec in pod %q: %w", pod, err)
	}
	return ExecResult{Stdout: stdout, Stderr: stderr, ExitCode: exitCode}, nil
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
