package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wizzz/pangolin-ingress-controller/internal/util"
)

func TestGenerateName(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		ingressName string
		host        string
		wantPrefix  string
	}{
		{
			name:        "standard case",
			namespace:   "default",
			ingressName: "myapp",
			host:        "app.example.com",
			wantPrefix:  "pic-default-myapp-",
		},
		{
			name:        "different namespace",
			namespace:   "production",
			ingressName: "api",
			host:        "api.example.com",
			wantPrefix:  "pic-production-api-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.GenerateName(tt.namespace, tt.ingressName, tt.host)

			// Check prefix
			assert.Contains(t, result, tt.wantPrefix)

			// Check total length is reasonable (63 char k8s limit)
			assert.LessOrEqual(t, len(result), 63)

			// Check deterministic (same input = same output)
			result2 := util.GenerateName(tt.namespace, tt.ingressName, tt.host)
			assert.Equal(t, result, result2)
		})
	}
}

func TestGenerateName_Deterministic(t *testing.T) {
	// Same inputs should always produce same output
	name1 := util.GenerateName("ns", "ingress", "host.example.com")
	name2 := util.GenerateName("ns", "ingress", "host.example.com")
	assert.Equal(t, name1, name2)
}

func TestGenerateName_Unique(t *testing.T) {
	// Different hosts should produce different names
	name1 := util.GenerateName("ns", "ingress", "host1.example.com")
	name2 := util.GenerateName("ns", "ingress", "host2.example.com")
	assert.NotEqual(t, name1, name2)
}

func TestGenerateName_ValidKubernetesName(t *testing.T) {
	name := util.GenerateName("my-namespace", "my-ingress", "app.example.com")

	// Must be lowercase
	assert.Equal(t, name, util.GenerateName("my-namespace", "my-ingress", "app.example.com"))

	// Must not start or end with hyphen
	assert.NotEqual(t, '-', name[0])
	assert.NotEqual(t, '-', name[len(name)-1])
}
