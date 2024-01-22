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
			want:     utils.UnstructuredManifest(),
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
