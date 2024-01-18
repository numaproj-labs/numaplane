package utils

import (
	"log"

	"gopkg.in/yaml.v2"
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
