package ssh

import "golang.org/x/crypto/ssh"

// applyLegacyCrypto widens cfg's algorithm negotiation to include
// obsolete key exchange, cipher, MAC, and host-key algorithms, so old
// switch/router SSH stacks that predate the modern secure defaults can
// still be reached. It appends rather than replaces the supported set, so
// negotiation still prefers modern algorithms when the target offers
// them. Never applied unless a target's inventory entry explicitly opts
// in (Target.LegacyCrypto) — see internal/inventory's NetworkDevice.
func applyLegacyCrypto(cfg *ssh.ClientConfig) {
	supported := ssh.SupportedAlgorithms()
	insecure := ssh.InsecureAlgorithms()

	cfg.Config = ssh.Config{
		KeyExchanges: append(append([]string{}, supported.KeyExchanges...), insecure.KeyExchanges...),
		Ciphers:      append(append([]string{}, supported.Ciphers...), insecure.Ciphers...),
		MACs:         append(append([]string{}, supported.MACs...), insecure.MACs...),
	}
	cfg.HostKeyAlgorithms = append(append([]string{}, supported.HostKeys...), insecure.HostKeys...)
}
