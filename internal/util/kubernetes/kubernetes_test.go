package kubernetes

import (
	"context"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	"github.com/numaproj-labs/numaplane/internal/common"
)

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

func TestGetGitSyncInstanceAnnotation(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	err = SetGitSyncInstanceAnnotation(&obj, common.AnnotationKeyGitSyncInstance, "my-gitsync")
	assert.Nil(t, err)

	annotation, err := GetGitSyncInstanceAnnotation(&obj, common.AnnotationKeyGitSyncInstance)
	assert.Nil(t, err)
	assert.Equal(t, "my-gitsync", annotation)
}

func TestGetGitSyncInstanceAnnotationWithInvalidData(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc-with-invalid-data.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	_, err = GetGitSyncInstanceAnnotation(&obj, "valid-annotation")
	assert.Error(t, err)
	assert.Equal(t, "failed to get annotations from target object /v1, Kind=Service /my-service: .metadata.annotations accessor error: contains non-string key in the map: <nil> is of the type <nil>, expected string", err.Error())
}

func TestIsValidKubernetesManifestFile(t *testing.T) {

	testCases := []struct {
		name         string
		resourceName string
		expected     bool
	}{
		{
			name:         "Invalid Name",
			resourceName: "data.md",
			expected:     false,
		},

		{
			name:         "valid name",
			resourceName: "my.yml",
			expected:     true,
		},

		{
			name:         "Valid Json file",
			resourceName: "pipeline.json",
			expected:     true,
		},

		{
			name:         "Valid name yaml",
			resourceName: "pipeline.yaml",
			expected:     true,
		},
		{
			name:         "Valid name yaml",
			resourceName: "pipeline.xyz.yaml",
			expected:     true,
		},
		{
			name:         "Valid name yaml",
			resourceName: "pipeline.xyz.hjk.json",
			expected:     true,
		},
		{
			name:         "Invalid File",
			resourceName: "main.go",
			expected:     false,
		},
		{
			name:         "Invalid File",
			resourceName: "main..json.go",
			expected:     false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := IsValidKubernetesManifestFile(tc.resourceName)
			assert.Equal(t, tc.expected, ok)
		})
	}

}

func TestApplyOwnerShipReference(t *testing.T) {
	resource := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: testing-deployment
  labels:
    app: testing-server
spec:
  replicas: 2
  selector:
    matchLabels:
      app: testing-server
  template:
    metadata:
      labels:
        app: testing-server
    spec:
      containers:
      - name: http-server
        image: nginx:latest
        ports:
        - containerPort: 80`

	gitsync := &v1alpha1.GitSync{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitSync",
			APIVersion: "numaplane.numaproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "gitsync-test", UID: "12345678-abcd-1234-ef00-1234567890ab"},
		Spec:       v1alpha1.GitSyncSpec{},
		Status:     v1alpha1.GitSyncStatus{},
	}
	reference, err := ApplyGitSyncOwnership(resource, gitsync)
	assert.Equal(t, `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: testing-server
  name: testing-deployment
  ownerReferences:
  - apiVersion: numaplane.numaproj.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: GitSync
    name: gitsync-test
    uid: 12345678-abcd-1234-ef00-1234567890ab
spec:
  replicas: 2
  selector:
    matchLabels:
      app: testing-server
  template:
    metadata:
      labels:
        app: testing-server
    spec:
      containers:
      - image: nginx:latest
        name: http-server
        ports:
        - containerPort: 80
`, string(reference))
	assert.NoError(t, err)

}

func TestApplyOwnerShipReferenceAppendExisting(t *testing.T) {
	resource := `apiVersion: v1
kind: Pod
metadata:
  name: my-custom-resource
  ownerReferences:
  - apiVersion: apps/v1
    kind: Deployment
    name: my-deployment
    uid: <uid-of-my-deployment>
    controller: false
    blockOwnerDeletion: true
  - apiVersion: v1
    kind: ConfigMap
    name: my-configmap
    uid: <uid-of-my-configmap>
    controller: false
    blockOwnerDeletion: true
`

	gitsync := &v1alpha1.GitSync{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitSync",
			APIVersion: "1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "gitsync-test", UID: "awew"},
		Spec:       v1alpha1.GitSyncSpec{},
		Status:     v1alpha1.GitSyncStatus{},
	}
	reference, err := ApplyGitSyncOwnership(resource, gitsync)
	assert.Equal(t, `apiVersion: v1
kind: Pod
metadata:
  name: my-custom-resource
  ownerReferences:
  - apiVersion: apps/v1
    blockOwnerDeletion: true
    controller: false
    kind: Deployment
    name: my-deployment
    uid: <uid-of-my-deployment>
  - apiVersion: v1
    blockOwnerDeletion: true
    controller: false
    kind: ConfigMap
    name: my-configmap
    uid: <uid-of-my-configmap>
  - apiVersion: "1"
    blockOwnerDeletion: true
    controller: true
    kind: GitSync
    name: gitsync-test
    uid: awew
`, string(reference))
	assert.NoError(t, err)

}

func TestApplyOwnerShipReferenceJSON(t *testing.T) {
	resource := `{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "nginx-deployment"
  },
  "spec": {
    "selector": {
      "matchLabels": {
        "app": "nginx"
      }
    },
    "replicas": 2,
    "template": {
      "metadata": {
        "labels": {
          "app": "nginx"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "nginx",
            "image": "nginx:1.14.2",
            "ports": [
              {
                "containerPort": 80
              }
            ]
          }
        ]
      }
    }
  }
}`

	gitsync := &v1alpha1.GitSync{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitSync",
			APIVersion: "1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "gitsync-test", UID: "awew"},
		Spec:       v1alpha1.GitSyncSpec{},
		Status:     v1alpha1.GitSyncStatus{},
	}
	reference, err := ApplyGitSyncOwnership(resource, gitsync)
	assert.Equal(t, `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  ownerReferences:
  - apiVersion: "1"
    blockOwnerDeletion: true
    controller: true
    kind: GitSync
    name: gitsync-test
    uid: awew
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx:1.14.2
        name: nginx
        ports:
        - containerPort: 80
`, string(reference))
	assert.NoError(t, err)

}

func TestGetSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	assert.NoError(t, err)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				"key": []byte("value"),
			},
		},
	).Build()

	ctx := context.TODO()

	secret, err := GetSecret(ctx, fakeClient, "test-namespace", "test-secret")

	assert.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "value", string(secret.Data["key"]))
}

func TestApplyOwnerShipReferenceDifferentNamespace(t *testing.T) {
	resource := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: testing-deployment
  namespace: test
  labels:
    app: testing-server
spec:
  replicas: 2
  selector:
    matchLabels:
      app: testing-server
  template:
    metadata:
      labels:
        app: testing-server
    spec:
      containers:
      - name: http-server
        image: nginx:latest
        ports:
        - containerPort: 80`

	gitsync := &v1alpha1.GitSync{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitSync",
			APIVersion: "numaplane.numaproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "gitsync-test", UID: "12345678-abcd-1234-ef00-1234567890ab", Namespace: "test-2"},
		Spec:       v1alpha1.GitSyncSpec{},
		Status:     v1alpha1.GitSyncStatus{},
	}
	_, err := ApplyGitSyncOwnership(resource, gitsync)
	assert.Error(t, err)
	assert.Equal(t, "GitSync object and the resource must be in the same namespace", err.Error())

}
