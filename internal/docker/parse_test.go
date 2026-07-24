package docker

import "testing"

func TestParsePs(t *testing.T) {
	input := `{"ID":"abc123","Image":"nginx:latest","Command":"\"nginx -g\"","CreatedAt":"2026-01-01 00:00:00","Status":"Up 2 hours","State":"running","Ports":"80/tcp","Names":"web","RunningFor":"2 hours ago"}
{"ID":"def456","Image":"redis:7","Command":"\"redis-server\"","CreatedAt":"2026-01-01 00:00:00","Status":"Exited (0) 1 hour ago","State":"exited","Ports":"","Names":"cache","RunningFor":"1 hour ago"}
`
	got, err := parsePs(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(got))
	}
	if got[0].ID != "abc123" || got[0].Names != "web" || got[0].State != "running" {
		t.Errorf("entry 0 = %+v", got[0])
	}
	if got[1].State != "exited" {
		t.Errorf("entry 1 = %+v", got[1])
	}
}

func TestParsePs_Empty(t *testing.T) {
	got, err := parsePs("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no containers, got %+v", got)
	}
}

func TestParsePs_MalformedJSON(t *testing.T) {
	_, err := parsePs("not json\n")
	if err == nil {
		t.Fatal("expected error for malformed JSON line")
	}
}

func TestParseImages(t *testing.T) {
	input := `{"ID":"sha256:abc","Repository":"nginx","Tag":"latest","CreatedAt":"2026-01-01 00:00:00","Size":"142MB"}
`
	got, err := parseImages(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Repository != "nginx" || got[0].Size != "142MB" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestParseImages_MalformedJSON(t *testing.T) {
	_, err := parseImages("not json\n")
	if err == nil {
		t.Fatal("expected error for malformed JSON line")
	}
}

func TestParseStats(t *testing.T) {
	input := `{"ID":"abc123","Name":"web","CPUPerc":"0.15%","MemUsage":"10MiB / 2GiB","MemPerc":"0.50%","NetIO":"1kB / 2kB","BlockIO":"0B / 0B","PIDs":"5"}
`
	got, err := parseStats(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "web" || got[0].CPUPercent != "0.15%" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestParseStats_MalformedJSON(t *testing.T) {
	_, err := parseStats("not json\n")
	if err == nil {
		t.Fatal("expected error for malformed JSON line")
	}
}
