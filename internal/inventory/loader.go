package inventory

import (
	"bytes"
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// Load reads, expands, parses, and validates the inventory file at path.
func Load(path string) (*Inventory, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("inventory: read %s: %w", path, err)
	}
	return Parse(raw)
}

// Parse decodes raw YAML bytes into a validated Inventory. Environment
// variable references (${VAR}) are expanded before parsing.
func Parse(raw []byte) (*Inventory, error) {
	expanded, err := expandEnv(raw)
	if err != nil {
		return nil, err
	}

	var inv Inventory
	dec := yaml.NewDecoder(bytes.NewReader(expanded))
	dec.KnownFields(true)
	if err := dec.Decode(&inv); err != nil {
		return nil, fmt.Errorf("inventory: parse yaml: %w", err)
	}

	if err := inv.Validate(); err != nil {
		return nil, err
	}

	return &inv, nil
}

// Validate checks structural correctness of every entry in the inventory.
// It is exported so callers that construct an Inventory programmatically
// (e.g. in tests) can validate it the same way Load does.
func (inv *Inventory) Validate() error {
	if err := validate.Struct(inv); err != nil {
		return fmt.Errorf("inventory: validation failed: %w", err)
	}

	for name, s := range inv.Servers {
		if err := validate.Struct(s); err != nil {
			return fmt.Errorf("inventory: server %q: %w", name, err)
		}
	}
	for name, r := range inv.Routers {
		if err := validate.Struct(r); err != nil {
			return fmt.Errorf("inventory: router %q: %w", name, err)
		}
	}
	for name, sw := range inv.Switches {
		if err := validate.Struct(sw); err != nil {
			return fmt.Errorf("inventory: switch %q: %w", name, err)
		}
	}
	if inv.Grafana != nil {
		if err := validate.Struct(inv.Grafana); err != nil {
			return fmt.Errorf("inventory: grafana: %w", err)
		}
	}
	if inv.Proxmox != nil {
		if err := validate.Struct(inv.Proxmox); err != nil {
			return fmt.Errorf("inventory: proxmox: %w", err)
		}
	}

	return nil
}
