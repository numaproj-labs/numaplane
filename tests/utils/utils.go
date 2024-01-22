package utils

import (
	"log"
	"time"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

func ParseYamlData() map[string]interface{} {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: nginx
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx
        name: nginx`)

	obj := make(map[string]interface{})
	if err := yaml.Unmarshal(data, obj); err != nil {
		log.Fatalf("failed to unmarshal yaml data, err: %v", err)
	}

	return obj
}

func UnstructuredManifest() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "nginx",
				"labels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "nginx",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"image": "nginx",
								"name":  "nginx",
							},
						},
					},
				},
			},
		},
	}
}

func GetTestRestConfig() *rest.Config {
	return &rest.Config{
		Host:    "localhost:8080",
		APIPath: "v1",
		ContentConfig: rest.ContentConfig{
			AcceptContentTypes: "application/json",
			ContentType:        "application/json",
		},

		BearerToken: "1234567890",
		Impersonate: rest.ImpersonationConfig{
			UserName: "gopher2",
			UID:      "uid123",
		},

		UserAgent: "gobot",
		QPS:       1,
		Burst:     2,
		Timeout:   3 * time.Second,
	}
}
