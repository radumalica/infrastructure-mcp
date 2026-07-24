package ssh

import (
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestApplyLegacyCrypto_WidensAlgorithmSets(t *testing.T) {
	cfg := &ssh.ClientConfig{}
	applyLegacyCrypto(cfg)

	insecure := ssh.InsecureAlgorithms()
	supported := ssh.SupportedAlgorithms()

	if len(cfg.KeyExchanges) <= len(supported.KeyExchanges) {
		t.Error("expected KeyExchanges to include insecure algorithms beyond the supported set")
	}
	if len(cfg.Ciphers) <= len(supported.Ciphers) {
		t.Error("expected Ciphers to include insecure algorithms beyond the supported set")
	}
	if len(cfg.HostKeyAlgorithms) <= len(supported.HostKeys) {
		t.Error("expected HostKeyAlgorithms to include insecure algorithms beyond the supported set")
	}

	if len(insecure.KeyExchanges) > 0 {
		found := false
		for _, alg := range cfg.KeyExchanges {
			if alg == insecure.KeyExchanges[0] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q to be present in the widened KeyExchanges list", insecure.KeyExchanges[0])
		}
	}
}

func TestApplyLegacyCrypto_DoesNotMutateSupportedAlgorithms(t *testing.T) {
	before := ssh.SupportedAlgorithms()
	cfg := &ssh.ClientConfig{}
	applyLegacyCrypto(cfg)
	after := ssh.SupportedAlgorithms()

	if len(before.KeyExchanges) != len(after.KeyExchanges) {
		t.Error("applyLegacyCrypto must not mutate the package-level supported algorithm set")
	}
}
