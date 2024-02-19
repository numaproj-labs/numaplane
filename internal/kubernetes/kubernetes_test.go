package kubernetes

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	mocksClient "github.com/numaproj-labs/numaplane/internal/kubernetes/mocks"
	"github.com/numaproj-labs/numaplane/tests/utils"
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

func Test_client_ApplyResource(t *testing.T) {
	type args struct {
		data              []byte
		namespaceOverride string
	}
	tests := []struct {
		name    string
		args    args
		err     error
		wantErr bool
	}{
		{
			name: "Failed to parse yaml file",
			args: args{
				data:              []byte("unexpected yaml data"),
				namespaceOverride: "numaflow-pipeline",
			},
			err:     fmt.Errorf("failed to unmarshal yaml data"),
			wantErr: true,
		},
		{
			name: "Failed to decode yaml file",
			args: args{
				data:              []byte("\"apiVersion\": \"apps/v1\",\n\t\t\t\"kind\":\"Deployment\","),
				namespaceOverride: "numaflow-pipeline",
			},
			err:     fmt.Errorf("failed to decode yaml"),
			wantErr: true,
		},
		{
			name: "Apply manifest successfully",
			args: args{
				data:              utils.GetTestManifest(),
				namespaceOverride: "numaflow-pipeline",
			},
			err:     nil,
			wantErr: false,
		},
	}

	t.Parallel()
	ctrl := gomock.NewController(t)
	c := mocksClient.NewMockClient(ctrl)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.EXPECT().ApplyResource(tt.args.data, tt.args.namespaceOverride).Return(tt.err)
			if err := c.ApplyResource(tt.args.data, tt.args.namespaceOverride); (err != nil) != tt.wantErr {
				Expect(err.Error()).To(Equal(tt.err))
				t.Errorf("ApplyResource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_client_DeleteResource(t *testing.T) {
	type args struct {
		kind      string
		name      string
		namespace string
		do        metav1.DeleteOptions
	}
	tests := []struct {
		name    string
		args    args
		err     error
		wantErr bool
	}{
		{
			name: "name is empty",
			args: args{
				kind:      "Deployment",
				name:      "",
				namespace: "numaflow-pipeline",
			},
			err:     fmt.Errorf("name is empty"),
			wantErr: true,
		},
		{
			name: "Delete resource",
			args: args{
				kind:      "Deployment",
				name:      "test-app",
				namespace: "numaflow-pipeline",
			},
			err:     nil,
			wantErr: false,
		},
	}

	ctrl := gomock.NewController(t)
	c := mocksClient.NewMockClient(ctrl)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.EXPECT().DeleteResource(tt.args.kind, tt.args.name, tt.args.namespace, tt.args.do).Return(tt.err)
			if err := c.DeleteResource(tt.args.kind, tt.args.name, tt.args.namespace, tt.args.do); (err != nil) != tt.wantErr {
				Expect(err.Error()).To(Equal(tt.err))
				t.Errorf("DeleteResource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetSecret(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	c := mocksClient.NewMockClient(ctrl)
	data := map[string][]byte{"username": []byte("admin"), "password": []byte("secret")}
	c.EXPECT().GetSecret(context.TODO(), "namespace", "secret").Return(&corev1.Secret{Data: data}, nil)
	secret, err := c.GetSecret(context.TODO(), "namespace", "secret")
	assert.NoError(t, err)
	assert.Equal(t, "admin", string(secret.Data["username"]))
}
