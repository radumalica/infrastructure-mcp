package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"infrastructure-mcp/internal/inventory"
	"infrastructure-mcp/internal/kubernetes"
)

type fakeKubeDiagnostics struct {
	pods    []kubernetes.PodEntry
	logs    string
	events  []kubernetes.EventEntry
	desc    kubernetes.PodDescription
	nodes   []kubernetes.NodeEntry
	err     error
	sawNS   string
	sawPod  string
	sawTail int64
	sawClu  string
}

func (f *fakeKubeDiagnostics) ListPods(_ context.Context, cluster, namespace string) ([]kubernetes.PodEntry, error) {
	f.sawClu, f.sawNS = cluster, namespace
	return f.pods, f.err
}
func (f *fakeKubeDiagnostics) Logs(_ context.Context, cluster, namespace, pod, _ string, tailLines int64) (string, error) {
	f.sawClu, f.sawNS, f.sawPod, f.sawTail = cluster, namespace, pod, tailLines
	return f.logs, f.err
}
func (f *fakeKubeDiagnostics) ListEvents(_ context.Context, cluster, namespace string) ([]kubernetes.EventEntry, error) {
	f.sawClu, f.sawNS = cluster, namespace
	return f.events, f.err
}
func (f *fakeKubeDiagnostics) DescribePod(_ context.Context, cluster, namespace, pod string) (kubernetes.PodDescription, error) {
	f.sawClu, f.sawNS, f.sawPod = cluster, namespace, pod
	return f.desc, f.err
}
func (f *fakeKubeDiagnostics) ListNodes(_ context.Context, cluster string) ([]kubernetes.NodeEntry, error) {
	f.sawClu = cluster
	return f.nodes, f.err
}

func newKubeSession(t *testing.T, diag *fakeKubeDiagnostics) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	RegisterKubectlGetPods(server, testLogger(), diag)
	RegisterKubectlLogs(server, testLogger(), diag)
	RegisterKubectlEvents(server, testLogger(), diag)
	RegisterKubectlDescribe(server, testLogger(), diag)
	RegisterKubectlNodes(server, testLogger(), diag)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	go func() { _, _ = server.Connect(ctx, serverTransport, nil) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestKubectlGetPods_ViaMCPProtocol(t *testing.T) {
	diag := &fakeKubeDiagnostics{pods: []kubernetes.PodEntry{
		{Name: "app-1", Namespace: "default", Phase: "Running", Ready: "1/1"},
	}}
	session := newKubeSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "kubectl_get_pods",
		Arguments: map[string]any{"cluster": "home", "namespace": "default"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawClu != "home" || diag.sawNS != "default" {
		t.Errorf("saw cluster=%q namespace=%q", diag.sawClu, diag.sawNS)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out KubectlGetPodsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Pods) != 1 || out.Pods[0].Name != "app-1" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestKubectlLogs_ViaMCPProtocol(t *testing.T) {
	diag := &fakeKubeDiagnostics{logs: "line1\nline2\n"}
	session := newKubeSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "kubectl_logs",
		Arguments: map[string]any{"cluster": "home", "namespace": "default", "pod": "app-1", "tail": 50},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawPod != "app-1" || diag.sawTail != 50 {
		t.Errorf("sawPod=%q sawTail=%d", diag.sawPod, diag.sawTail)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out KubectlLogsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Logs != "line1\nline2\n" || out.LineCount != 2 {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestKubectlEvents_ViaMCPProtocol(t *testing.T) {
	diag := &fakeKubeDiagnostics{events: []kubernetes.EventEntry{
		{Type: "Warning", Reason: "Failed", Object: "Pod/app-1", Message: "boom"},
	}}
	session := newKubeSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "kubectl_events",
		Arguments: map[string]any{"cluster": "home"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out KubectlEventsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Events) != 1 || out.Events[0].Reason != "Failed" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestKubectlDescribe_ViaMCPProtocol(t *testing.T) {
	diag := &fakeKubeDiagnostics{desc: kubernetes.PodDescription{
		Name:      "app-1",
		Namespace: "default",
		Phase:     "Running",
		Containers: []kubernetes.ContainerStatus{
			{Name: "app", Ready: true, Image: "nginx:latest", State: "running"},
		},
	}}
	session := newKubeSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "kubectl_describe",
		Arguments: map[string]any{"cluster": "home", "namespace": "default", "pod": "app-1"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawPod != "app-1" {
		t.Errorf("sawPod = %q, want app-1", diag.sawPod)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out KubectlDescribeOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Containers) != 1 || out.Containers[0].Image != "nginx:latest" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestKubectlNodes_ViaMCPProtocol(t *testing.T) {
	diag := &fakeKubeDiagnostics{nodes: []kubernetes.NodeEntry{
		{Name: "node-a", Ready: true, Roles: []string{"control-plane"}},
	}}
	session := newKubeSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "kubectl_nodes",
		Arguments: map[string]any{"cluster": "home"},
	})
	if err != nil || result.IsError {
		t.Fatalf("CallTool failed: err=%v result=%+v", err, result)
	}
	if diag.sawClu != "home" {
		t.Errorf("sawClu = %q, want home", diag.sawClu)
	}

	raw, _ := json.Marshal(result.StructuredContent)
	var out KubectlNodesOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Nodes) != 1 || out.Nodes[0].Name != "node-a" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestKubernetesTools_ErrorPath(t *testing.T) {
	diag := &fakeKubeDiagnostics{err: errors.New("boom")}
	session := newKubeSession(t, diag)
	ctx := context.Background()

	cases := []struct {
		tool string
		args map[string]any
	}{
		{"kubectl_get_pods", map[string]any{"cluster": "home"}},
		{"kubectl_logs", map[string]any{"cluster": "home", "namespace": "default", "pod": "app-1"}},
		{"kubectl_events", map[string]any{"cluster": "home"}},
		{"kubectl_describe", map[string]any{"cluster": "home", "namespace": "default", "pod": "app-1"}},
		{"kubectl_nodes", map[string]any{"cluster": "home"}},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
			if err != nil {
				t.Fatalf("CallTool transport error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected an error result for %s", tc.tool)
			}
		})
	}
}

func TestKubectlGetPods_ClusterNotFound(t *testing.T) {
	diag := &fakeKubeDiagnostics{err: inventory.ErrNotFound}
	session := newKubeSession(t, diag)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "kubectl_get_pods",
		Arguments: map[string]any{"cluster": "does-not-exist"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for a missing cluster")
	}
}
