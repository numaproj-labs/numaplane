package utils

import (
	"log"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ParseYamlData() map[string]interface{} {
	obj := make(map[string]interface{})
	if err := yaml.Unmarshal(GetTestManifest(), obj); err != nil {
		log.Fatalf("failed to unmarshal yaml data, err: %v", err)
	}

	return obj
}

func GetTestManifest() []byte {
	return []byte(`apiVersion: apps/v1
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
