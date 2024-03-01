package kubernetes

import "testing"

func TestIsValidKubernetesNamespace(t *testing.T) {
	testCases := []struct {
		name     string
		expected bool
	}{
		{"valid-namespace", true},
		{"INVALID", false},
		{"kubernetes-namespace", false},
		{"kube-system", false},
		{"1234", true},
		{"valid123", true},
		{"valid.namespace", false},
		{"-invalid", false},
		{"invalid-", false},
		{"valid-namespace-with-long-name-123456789012345678901234567890123456789012345678901234567890123", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := IsValidKubernetesNamespace(tc.name)
			if actual != tc.expected {
				t.Errorf("For namespace '%s', expected %v but got %v", tc.name, tc.expected, actual)
			}
		})
	}
}
