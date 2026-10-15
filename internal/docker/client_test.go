package docker

import (
	"context"
	"errors"
	"testing"

	"infrastructure-mcp/internal/ssh"
)

// fakeRunner is a scripted Runner: each server+command pair maps to a
// canned ssh.Result or error. Mirrors internal/linux's client_test.go
// fake.
type fakeRunner struct {
	results map[string]ssh.Result
	errs    map[string]error
}

func (f *fakeRunner) Run(_ context.Context, server, command string) (ssh.Result, error) {
	key := server + "|" + command
	if err, ok := f.errs[key]; ok {
		return ssh.Result{}, err
	}
	if res, ok := f.results[key]; ok {
		return res, nil
	}
	return ssh.Result{}, errors.New("fakeRunner: no script for " + key)
}

func TestClient_Ps(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker ps --format '{{json .}}'": {
			Stdout:   `{"ID":"abc123","Image":"nginx","Names":"web","State":"running"}` + "\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.Ps(context.Background(), "archive", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Names != "web" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestClient_Ps_All(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker ps --all --format '{{json .}}'": {
			Stdout:   `{"ID":"abc123","Image":"nginx","Names":"web","State":"exited"}` + "\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.Ps(context.Background(), "archive", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].State != "exited" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestClient_Ps_NonZeroExit(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker ps --format '{{json .}}'": {ExitCode: 1, Stderr: "docker: command not found"},
	}}
	c := New(runner)

	if _, err := c.Ps(context.Background(), "archive", false); err == nil {
		t.Fatal("expected error for nonzero exit")
	}
}

func TestClient_Images(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker images --format '{{json .}}'": {
			Stdout:   `{"ID":"sha256:abc","Repository":"nginx","Tag":"latest","Size":"142MB"}` + "\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.Images(context.Background(), "archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Repository != "nginx" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestClient_Stats_AllContainers(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker stats --no-stream --format '{{json .}}'": {
			Stdout:   `{"ID":"abc123","Name":"web","CPUPerc":"1.0%"}` + "\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.Stats(context.Background(), "archive", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "web" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestClient_Stats_SingleContainer(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker stats --no-stream --format '{{json .}}' web": {
			Stdout:   `{"ID":"abc123","Name":"web","CPUPerc":"1.0%"}` + "\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.Stats(context.Background(), "archive", "web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestClient_Stats_InvalidContainer(t *testing.T) {
	c := New(&fakeRunner{})
	if _, err := c.Stats(context.Background(), "archive", "web; rm -rf /"); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestClient_Logs(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker logs --tail 100 web 2>&1": {
			Stdout:   "line1\nline2\n",
			ExitCode: 0,
		},
	}}
	c := New(runner)

	got, err := c.Logs(context.Background(), "archive", "web", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "line1\nline2\n" {
		t.Errorf("Logs() = %q", got)
	}
}

func TestClient_Logs_TailClamping(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker logs --tail 5000 web 2>&1": {Stdout: "log\n", ExitCode: 0},
	}}
	c := New(runner)

	if _, err := c.Logs(context.Background(), "archive", "web", 999999); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Logs_InvalidContainer(t *testing.T) {
	c := New(&fakeRunner{})
	if _, err := c.Logs(context.Background(), "archive", "$(id)", 10); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestClient_Logs_NonZeroExit(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker logs --tail 100 web 2>&1": {ExitCode: 1, Stdout: "Error: No such container: web"},
	}}
	c := New(runner)

	if _, err := c.Logs(context.Background(), "archive", "web", 0); err == nil {
		t.Fatal("expected error for nonzero exit")
	}
}

func TestClient_Restart(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker restart web": {Stdout: "web\n", ExitCode: 0},
	}}
	c := New(runner)

	if err := c.Restart(context.Background(), "archive", "web"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Restart_InvalidContainer(t *testing.T) {
	c := New(&fakeRunner{})
	if err := c.Restart(context.Background(), "archive", "web; rm -rf /"); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestClient_Restart_NonZeroExit(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker restart web": {ExitCode: 1, Stderr: "Error: No such container: web"},
	}}
	c := New(runner)

	if err := c.Restart(context.Background(), "archive", "web"); err == nil {
		t.Fatal("expected error for nonzero exit")
	}
}

func TestClient_Exec(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker exec web sh -c 'echo hi'": {Stdout: "hi\n", ExitCode: 0},
	}}
	c := New(runner)

	got, err := c.Exec(context.Background(), "archive", "web", "echo hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Stdout != "hi\n" || got.ExitCode != 0 {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestClient_Exec_QuotesEmbeddedSingleQuotes(t *testing.T) {
	// shellQuoteSingle("it's ok") == "'it'\''s ok'", so the full remote
	// command is: docker exec web sh -c 'it'\''s ok'
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker exec web sh -c 'it'\\''s ok'": {Stdout: "it's ok\n", ExitCode: 0},
	}}
	c := New(runner)

	got, err := c.Exec(context.Background(), "archive", "web", "it's ok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Stdout != "it's ok\n" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestClient_Exec_NonZeroExitIsNotAnError(t *testing.T) {
	runner := &fakeRunner{results: map[string]ssh.Result{
		"archive|docker exec web sh -c 'false'": {ExitCode: 1},
	}}
	c := New(runner)

	got, err := c.Exec(context.Background(), "archive", "web", "false")
	if err != nil {
		t.Fatalf("expected no error for a non-zero exit, exit code is data: %v", err)
	}
	if got.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", got.ExitCode)
	}
}

func TestClient_Exec_InvalidContainer(t *testing.T) {
	c := New(&fakeRunner{})
	if _, err := c.Exec(context.Background(), "archive", "web; rm -rf /", "echo hi"); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestClient_RunnerError(t *testing.T) {
	runner := &fakeRunner{}
	c := New(runner)

	if _, err := c.Ps(context.Background(), "unscripted", false); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
	if _, err := c.Images(context.Background(), "unscripted"); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
	if _, err := c.Stats(context.Background(), "unscripted", ""); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
	if _, err := c.Logs(context.Background(), "unscripted", "web", 0); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
	if err := c.Restart(context.Background(), "unscripted", "web"); err == nil {
		t.Fatal("expected error when runner has no script for the call")
	}
}
