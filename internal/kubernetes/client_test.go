package kubernetes

import (
	"context"
	"errors"
	"io"
	neturl "net/url"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"infrastructure-mcp/internal/inventory"
)

// fakeLookup satisfies ClusterLookup but is never expected to be called
// in these tests: the fake clientset is pre-seeded into Client.conns
// directly, bypassing real kubeconfig loading entirely.
type fakeLookup struct{}

func (fakeLookup) KubeCluster(name string) (inventory.KubeCluster, error) {
	return inventory.KubeCluster{}, errors.New("fakeLookup: should not be called when the clientset is pre-seeded")
}

func testClient(cs kubeclient.Interface) *Client {
	return &Client{
		inv:   fakeLookup{},
		conns: map[string]clusterConn{"test": {clientset: cs}},
	}
}

func TestListPods(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-a"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", Ready: true, RestartCount: 2},
			},
		},
	}
	c := testClient(k8sfake.NewSimpleClientset(pod))

	pods, err := c.ListPods(context.Background(), "test", "default")
	if err != nil {
		t.Fatalf("ListPods: %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	if pods[0].Name != "app-1" || pods[0].Ready != "1/1" || pods[0].Restarts != 2 {
		t.Errorf("unexpected pod entry: %+v", pods[0])
	}
}

func TestListPods_ClusterNotFound(t *testing.T) {
	c := New(&inventory.Inventory{})
	_, err := c.ListPods(context.Background(), "does-not-exist", "")
	if !errors.Is(err, inventory.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestLogs(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"}}
	c := testClient(k8sfake.NewSimpleClientset(pod))

	out, err := c.Logs(context.Background(), "test", "default", "app-1", "app", 50)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty log output")
	}
}

func TestListEvents_SortedMostRecentFirst(t *testing.T) {
	older := metav1.NewTime(mustParseTime(t, "2026-01-01T00:00:00Z"))
	newer := metav1.NewTime(mustParseTime(t, "2026-06-01T00:00:00Z"))

	e1 := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "evt-old", Namespace: "default"},
		Type:           "Warning",
		Reason:         "Failed",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "app-1"},
		LastTimestamp:  older,
	}
	e2 := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "evt-new", Namespace: "default"},
		Type:           "Normal",
		Reason:         "Scheduled",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "app-1"},
		LastTimestamp:  newer,
	}
	c := testClient(k8sfake.NewSimpleClientset(e1, e2))

	events, err := c.ListEvents(context.Background(), "test", "default")
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Reason != "Scheduled" {
		t.Errorf("expected most recent event first, got %+v", events[0])
	}
}

func TestListNodes(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-a",
			Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
		},
		Spec: corev1.NodeSpec{Unschedulable: false},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.5"},
			},
			NodeInfo: corev1.NodeSystemInfo{KubeletVersion: "v1.30.0"},
		},
	}
	c := testClient(k8sfake.NewSimpleClientset(node))

	nodes, err := c.ListNodes(context.Background(), "test")
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	if !n.Ready || n.InternalIP != "10.0.0.5" || len(n.Roles) != 1 || n.Roles[0] != "control-plane" {
		t.Errorf("unexpected node entry: %+v", n)
	}
}

func TestDescribePod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-a"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.1.1.1",
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "app",
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
		},
	}
	c := testClient(k8sfake.NewSimpleClientset(pod))

	desc, err := c.DescribePod(context.Background(), "test", "default", "app-1")
	if err != nil {
		t.Fatalf("DescribePod: %v", err)
	}
	if desc.PodIP != "10.1.1.1" || len(desc.Containers) != 1 {
		t.Fatalf("unexpected description: %+v", desc)
	}
	if desc.Containers[0].State != "waiting" || desc.Containers[0].Reason != "CrashLoopBackOff" {
		t.Errorf("unexpected container status: %+v", desc.Containers[0])
	}
}

func TestClientFor_InvalidKubeconfigPath(t *testing.T) {
	inv := &inventory.Inventory{
		Kubernetes: map[string]inventory.KubeCluster{
			"broken": {Kubeconfig: "/does/not/exist"},
		},
	}
	c := New(inv)
	_, err := c.ListPods(context.Background(), "broken", "default")
	if err == nil {
		t.Fatal("expected an error for a nonexistent kubeconfig path")
	}
}

func TestClientFor_CachesClientset(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"}}
	cs := k8sfake.NewSimpleClientset(pod)
	c := testClient(cs)

	first, err := c.clientFor("test")
	if err != nil {
		t.Fatalf("clientFor: %v", err)
	}
	second, err := c.clientFor("test")
	if err != nil {
		t.Fatalf("clientFor: %v", err)
	}
	if first != second {
		t.Error("expected clientFor to return the cached clientset on the second call")
	}
}

func TestDescribePod_NotFound(t *testing.T) {
	c := testClient(k8sfake.NewSimpleClientset())
	_, err := c.DescribePod(context.Background(), "test", "default", "does-not-exist")
	if err == nil {
		t.Fatal("expected an error for a missing pod")
	}
}

// TestExec substitutes Client.stream with a fake: the fake clientset used
// everywhere else in this file has no real exec sub-resource (it's a raw
// HTTP connection upgrade, not a REST verb a reactor can intercept), so
// Exec's request-building/URL-plumbing is exercised for real while the
// actual SPDY stream is faked, mirroring how internal/docker's tests fake
// the Runner rather than a real SSH connection.
// testClientWithConfig is like testClient but also seeds a usable
// *rest.Config, needed by Exec (which builds its own REST client, unlike
// every other method here which only needs the clientset).
func testClientWithConfig(cs kubeclient.Interface) *Client {
	return &Client{
		inv:   fakeLookup{},
		conns: map[string]clusterConn{"test": {clientset: cs, restConfig: &rest.Config{Host: "https://127.0.0.1:6443"}}},
	}
}

func TestExec(t *testing.T) {
	cs := k8sfake.NewSimpleClientset()
	c := testClientWithConfig(cs)
	var gotMethod string
	var gotPath string
	c.stream = func(_ context.Context, _ *rest.Config, method string, u *neturl.URL, _ io.Reader) (string, string, int, error) {
		gotMethod = method
		gotPath = u.Path
		return "hi\n", "", 0, nil
	}

	got, err := c.Exec(context.Background(), "test", "default", "app-1", "app", []string{"echo", "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Stdout != "hi\n" || got.ExitCode != 0 {
		t.Errorf("unexpected result: %+v", got)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath == "" || !strings.Contains(gotPath, "/exec") {
		t.Errorf("unexpected request path: %q", gotPath)
	}
}

func TestExec_NonZeroExitIsNotAnError(t *testing.T) {
	c := testClientWithConfig(k8sfake.NewSimpleClientset())
	c.stream = func(context.Context, *rest.Config, string, *neturl.URL, io.Reader) (string, string, int, error) {
		return "", "boom\n", 1, nil
	}

	got, err := c.Exec(context.Background(), "test", "default", "app-1", "app", []string{"false"})
	if err != nil {
		t.Fatalf("expected no error for a non-zero exit, exit code is data: %v", err)
	}
	if got.ExitCode != 1 || got.Stderr != "boom\n" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestExec_StreamError(t *testing.T) {
	c := testClientWithConfig(k8sfake.NewSimpleClientset())
	c.stream = func(context.Context, *rest.Config, string, *neturl.URL, io.Reader) (string, string, int, error) {
		return "", "", 0, errors.New("connection refused")
	}

	if _, err := c.Exec(context.Background(), "test", "default", "app-1", "app", []string{"echo", "hi"}); err == nil {
		t.Fatal("expected an error when the stream fails")
	}
}

func TestExec_ClusterNotFound(t *testing.T) {
	c := New(fakeLookup{})
	if _, err := c.Exec(context.Background(), "does-not-exist", "default", "app-1", "app", []string{"echo", "hi"}); err == nil {
		t.Fatal("expected an error for an unknown cluster")
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed
}
