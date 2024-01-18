package kubernetes

import (
	"reflect"
	"testing"

	"github.com/numaproj-labs/numaplane/tests/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestToUnstructured(t *testing.T) {
	tests := []struct {
		name     string
		manifest map[string]interface{}
		want     *unstructured.Unstructured
		wantErr  bool
	}{
		{
			name:     "Successfully parsed data",
			manifest: utils.ParseYamlData(),
			want:     unstructuredExpectedSuccessful(),
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToUnstructured(tt.manifest)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToUnstructured() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToUnstructured() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func unstructuredExpectedSuccessful() *unstructured.Unstructured {
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
