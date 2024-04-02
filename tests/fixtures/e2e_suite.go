/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fixtures

import (
	"context"
	"os"
	"time"

	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
	planeversiond "github.com/numaproj-labs/numaplane/pkg/client/clientset/versioned"
	planepkg "github.com/numaproj-labs/numaplane/pkg/client/clientset/versioned/typed/numaplane/v1alpha1"
)

const (
	/* resource names */
	Namespace       = "numaplane-system"
	TargetNamespace = "numaflow-pipeline"
	E2ELabel        = "numaplane-e2e"
	E2ELabelValue   = "true"
	defaultTimeout  = 60 * time.Second
)

var (
	background = metav1.DeletePropagationBackground
)

type E2ESuite struct {
	suite.Suite
	restConfig    *rest.Config
	kubeClient    kubernetes.Interface
	gitSyncClient planepkg.GitSyncInterface
	stopch        chan struct{}
}

func (s *E2ESuite) SetupSuite() {
	var err error
	s.stopch = make(chan struct{})
	s.restConfig, err = k8sRestConfig()
	s.CheckError(err)
	s.kubeClient, err = kubernetes.NewForConfig(s.restConfig)
	s.CheckError(err)

	s.gitSyncClient = planeversiond.NewForConfigOrDie(s.restConfig).NumaplaneV1alpha1().GitSyncs(Namespace)

	// resource cleanup
	/*
		s.deleteResources([]schema.GroupVersionResource{
			v1alpha1.GitSyncGroupVersionResource,
		})

	*/

	// port forward git server pod
	err = PodPortForward(s.restConfig, Namespace, "localgitserver-0", 8080, 80, s.stopch)
	s.CheckError(err)

}

func (s *E2ESuite) TearDownSuite() {
	s.deleteResources([]schema.GroupVersionResource{
		v1alpha1.GitSyncGroupVersionResource,
	})
	close(s.stopch)
}

func (s *E2ESuite) BeforeTest() {}

func (s *E2ESuite) AfterTest(suiteName, testName string) {
	// delete tmp directory after each test
	err := os.RemoveAll("local")
	s.CheckError(err)
}

func (s *E2ESuite) CheckError(err error) {
	s.T().Helper()
	if err != nil {
		s.T().Fatal(err)
	}
}

func (s *E2ESuite) Given() *Given {
	return &Given{
		t:             s.T(),
		restConfig:    s.restConfig,
		kubeClient:    s.kubeClient,
		gitSyncClient: s.gitSyncClient,
	}
}

func (s *E2ESuite) deleteResources(resources []schema.GroupVersionResource) {
	hasTestLabel := metav1.ListOptions{LabelSelector: E2ELabel}
	ctx := context.Background()
	for _, r := range resources {
		err := s.dynamicFor(r).DeleteCollection(ctx, metav1.DeleteOptions{PropagationPolicy: &background}, hasTestLabel)
		s.CheckError(err)
	}

	for _, r := range resources {
		for {
			list, err := s.dynamicFor(r).List(ctx, hasTestLabel)
			s.CheckError(err)
			if len(list.Items) == 0 {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (s *E2ESuite) dynamicFor(r schema.GroupVersionResource) dynamic.ResourceInterface {
	resourceInterface := dynamic.NewForConfigOrDie(s.restConfig).Resource(r)
	return resourceInterface.Namespace(Namespace)
}

// k8sRestConfig returns a rest config for the kubernetes cluster.
// TODO: move to different file if needs to be shared
func k8sRestConfig() (*rest.Config, error) {
	var restConfig *rest.Config
	var err error
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = home + "/.kube/config"
		if _, err := os.Stat(kubeconfig); err != nil && os.IsNotExist(err) {
			kubeconfig = ""
		}
	}
	if kubeconfig != "" {
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		restConfig, err = rest.InClusterConfig()
	}
	return restConfig, err
}
