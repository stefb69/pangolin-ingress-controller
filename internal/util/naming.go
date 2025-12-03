package util

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// NamePrefix is the prefix for all PIC-managed resources.
	NamePrefix = "pic"

	// MaxNameLength is the maximum length for Kubernetes resource names.
	MaxNameLength = 63
)

// GenerateName creates a deterministic resource name for a PangolinResource.
//
// # Algorithm
//
// The name is generated using the format: pic-<namespace>-<ingress>-<hash>
//
// The hash is computed from: SHA256(namespace + "/" + ingressName + "/" + host)
// Only the first 4 bytes (8 hex characters) are used for brevity.
//
// # Multi-Host Support
//
// This function is designed to support multi-host Ingresses. Each unique host
// in an Ingress will produce a different hash, ensuring that:
//   - Different hosts in the same Ingress get different PangolinResource names
//   - The same host always produces the same name (idempotent)
//   - Names are deterministic and predictable for GitOps workflows
//
// # Examples
//
//	GenerateName("default", "myapp", "app.example.com")  -> "pic-default-myapp-a1b2c3d4"
//	GenerateName("default", "myapp", "api.example.com")  -> "pic-default-myapp-e5f6g7h8"
//	GenerateName("prod", "myapp", "app.example.com")     -> "pic-prod-myapp-i9j0k1l2"
//
// # Kubernetes Name Constraints
//
// The generated name is sanitized to comply with Kubernetes naming rules:
//   - Maximum 63 characters
//   - Lowercase alphanumeric and hyphens only
//   - Cannot start or end with a hyphen
func GenerateName(namespace, ingressName, host string) string {
	// Create hash from all components for uniqueness
	hashInput := fmt.Sprintf("%s/%s/%s", namespace, ingressName, host)
	hash := sha256.Sum256([]byte(hashInput))
	shortHash := hex.EncodeToString(hash[:4]) // 8 characters

	// Build the name
	name := fmt.Sprintf("%s-%s-%s-%s", NamePrefix, namespace, ingressName, shortHash)

	// Ensure it's a valid Kubernetes name
	name = sanitizeName(name)

	// Truncate if necessary (keep the hash at the end)
	if len(name) > MaxNameLength {
		// Keep prefix and hash, truncate middle
		prefixLen := len(NamePrefix) + 1 + len(namespace) + 1 // "pic-namespace-"
		suffixLen := len(shortHash) + 1                       // "-hash"
		availableLen := MaxNameLength - prefixLen - suffixLen

		if availableLen > 0 {
			truncatedIngress := ingressName
			if len(truncatedIngress) > availableLen {
				truncatedIngress = truncatedIngress[:availableLen]
			}
			name = fmt.Sprintf("%s-%s-%s-%s", NamePrefix, namespace, truncatedIngress, shortHash)
		} else {
			// Extreme case: just use prefix and hash
			name = fmt.Sprintf("%s-%s", NamePrefix, shortHash)
		}
	}

	return name
}

// sanitizeName ensures the name is a valid Kubernetes resource name.
func sanitizeName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with hyphens
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}
	name = result.String()

	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Collapse multiple hyphens
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	return name
}
