package agent

import (
	"context"

	"go.uber.org/zap"

	apiv1 "github.com/numaproj-labs/numaplane/api/v1alpha1"

	kubeutil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AgentSyncer struct {
	logger    *zap.SugaredLogger
	client    client.Client
	config    *rest.Config
	rawConfig *rest.Config
	kubectl   kubeutil.Kubectl

	//stateCache  LiveStateCache
}

func (syncer *AgentSyncer) Run(ctx context.Context) error {

	var err error
	var source apiv1.CredentialedGitSource
	configMapRevision := -1
	configManager := GetConfigManagerInstance()
	var configMap AgentConfig

	for {
		select {
		default:
			// Reload our copy of the ConfigMap if it changed (or load it the first time upon starting)
			if configManager.GetRevisionIndex() > configMapRevision {
				configMap, configMapRevision, err = configManager.GetConfig()
				if err != nil {
					syncer.logger.Error(err)
					continue
				}

				// regenerate source

			}

			// clone/fetch repo
			// apply resource

			//time.Sleep(???)
		case <-ctx.Done():
			syncer.logger.Info("context ended, terminating AgentSyncer watch")
			return nil
		}
	}

}
