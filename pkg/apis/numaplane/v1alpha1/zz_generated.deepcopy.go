//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CommitStatus) DeepCopyInto(out *CommitStatus) {
	*out = *in
	in.SyncTime.DeepCopyInto(&out.SyncTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CommitStatus.
func (in *CommitStatus) DeepCopy() *CommitStatus {
	if in == nil {
		return nil
	}
	out := new(CommitStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Controller) DeepCopyInto(out *Controller) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Controller.
func (in *Controller) DeepCopy() *Controller {
	if in == nil {
		return nil
	}
	out := new(Controller)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerRollout) DeepCopyInto(out *ControllerRollout) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerRollout.
func (in *ControllerRollout) DeepCopy() *ControllerRollout {
	if in == nil {
		return nil
	}
	out := new(ControllerRollout)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ControllerRollout) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerRolloutList) DeepCopyInto(out *ControllerRolloutList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ControllerRollout, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerRolloutList.
func (in *ControllerRolloutList) DeepCopy() *ControllerRolloutList {
	if in == nil {
		return nil
	}
	out := new(ControllerRolloutList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ControllerRolloutList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerRolloutSpec) DeepCopyInto(out *ControllerRolloutSpec) {
	*out = *in
	out.Controller = in.Controller
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerRolloutSpec.
func (in *ControllerRolloutSpec) DeepCopy() *ControllerRolloutSpec {
	if in == nil {
		return nil
	}
	out := new(ControllerRolloutSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerRolloutStatus) DeepCopyInto(out *ControllerRolloutStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerRolloutStatus.
func (in *ControllerRolloutStatus) DeepCopy() *ControllerRolloutStatus {
	if in == nil {
		return nil
	}
	out := new(ControllerRolloutStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CredentialedGitLocation) DeepCopyInto(out *CredentialedGitLocation) {
	*out = *in
	out.GitLocation = in.GitLocation
	if in.RepoCredential != nil {
		in, out := &in.RepoCredential, &out.RepoCredential
		*out = new(RepoCredential)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CredentialedGitLocation.
func (in *CredentialedGitLocation) DeepCopy() *CredentialedGitLocation {
	if in == nil {
		return nil
	}
	out := new(CredentialedGitLocation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CredentialedGitSource) DeepCopyInto(out *CredentialedGitSource) {
	*out = *in
	in.GitSource.DeepCopyInto(&out.GitSource)
	if in.RepoCredential != nil {
		in, out := &in.RepoCredential, &out.RepoCredential
		*out = new(RepoCredential)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CredentialedGitSource.
func (in *CredentialedGitSource) DeepCopy() *CredentialedGitSource {
	if in == nil {
		return nil
	}
	out := new(CredentialedGitSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Destination) DeepCopyInto(out *Destination) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Destination.
func (in *Destination) DeepCopy() *Destination {
	if in == nil {
		return nil
	}
	out := new(Destination)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FileKeySelector) DeepCopyInto(out *FileKeySelector) {
	*out = *in
	if in.JSONFilePath != nil {
		in, out := &in.JSONFilePath, &out.JSONFilePath
		*out = new(string)
		**out = **in
	}
	if in.YAMLFilePath != nil {
		in, out := &in.YAMLFilePath, &out.YAMLFilePath
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FileKeySelector.
func (in *FileKeySelector) DeepCopy() *FileKeySelector {
	if in == nil {
		return nil
	}
	out := new(FileKeySelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitLocation) DeepCopyInto(out *GitLocation) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitLocation.
func (in *GitLocation) DeepCopy() *GitLocation {
	if in == nil {
		return nil
	}
	out := new(GitLocation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitSource) DeepCopyInto(out *GitSource) {
	*out = *in
	out.GitLocation = in.GitLocation
	if in.Kustomize != nil {
		in, out := &in.Kustomize, &out.Kustomize
		*out = new(KustomizeSource)
		**out = **in
	}
	if in.Helm != nil {
		in, out := &in.Helm, &out.Helm
		*out = new(HelmSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Raw != nil {
		in, out := &in.Raw, &out.Raw
		*out = new(RawSource)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitSource.
func (in *GitSource) DeepCopy() *GitSource {
	if in == nil {
		return nil
	}
	out := new(GitSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitSync) DeepCopyInto(out *GitSync) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitSync.
func (in *GitSync) DeepCopy() *GitSync {
	if in == nil {
		return nil
	}
	out := new(GitSync)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GitSync) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitSyncList) DeepCopyInto(out *GitSyncList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]GitSync, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitSyncList.
func (in *GitSyncList) DeepCopy() *GitSyncList {
	if in == nil {
		return nil
	}
	out := new(GitSyncList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GitSyncList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitSyncSpec) DeepCopyInto(out *GitSyncSpec) {
	*out = *in
	in.GitSource.DeepCopyInto(&out.GitSource)
	out.Destination = in.Destination
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitSyncSpec.
func (in *GitSyncSpec) DeepCopy() *GitSyncSpec {
	if in == nil {
		return nil
	}
	out := new(GitSyncSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitSyncStatus) DeepCopyInto(out *GitSyncStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.CommitStatus != nil {
		in, out := &in.CommitStatus, &out.CommitStatus
		*out = new(CommitStatus)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitSyncStatus.
func (in *GitSyncStatus) DeepCopy() *GitSyncStatus {
	if in == nil {
		return nil
	}
	out := new(GitSyncStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPCredential) DeepCopyInto(out *HTTPCredential) {
	*out = *in
	in.Password.DeepCopyInto(&out.Password)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPCredential.
func (in *HTTPCredential) DeepCopy() *HTTPCredential {
	if in == nil {
		return nil
	}
	out := new(HTTPCredential)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HelmParameter) DeepCopyInto(out *HelmParameter) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HelmParameter.
func (in *HelmParameter) DeepCopy() *HelmParameter {
	if in == nil {
		return nil
	}
	out := new(HelmParameter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HelmSource) DeepCopyInto(out *HelmSource) {
	*out = *in
	if in.ValueFiles != nil {
		in, out := &in.ValueFiles, &out.ValueFiles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = make([]HelmParameter, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HelmSource.
func (in *HelmSource) DeepCopy() *HelmSource {
	if in == nil {
		return nil
	}
	out := new(HelmSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ISBServiceRollout) DeepCopyInto(out *ISBServiceRollout) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ISBServiceRollout.
func (in *ISBServiceRollout) DeepCopy() *ISBServiceRollout {
	if in == nil {
		return nil
	}
	out := new(ISBServiceRollout)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ISBServiceRollout) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ISBServiceRolloutList) DeepCopyInto(out *ISBServiceRolloutList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ISBServiceRollout, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ISBServiceRolloutList.
func (in *ISBServiceRolloutList) DeepCopy() *ISBServiceRolloutList {
	if in == nil {
		return nil
	}
	out := new(ISBServiceRolloutList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ISBServiceRolloutList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ISBServiceRolloutSpec) DeepCopyInto(out *ISBServiceRolloutSpec) {
	*out = *in
	in.InterStepBufferService.DeepCopyInto(&out.InterStepBufferService)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ISBServiceRolloutSpec.
func (in *ISBServiceRolloutSpec) DeepCopy() *ISBServiceRolloutSpec {
	if in == nil {
		return nil
	}
	out := new(ISBServiceRolloutSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ISBServiceRolloutStatus) DeepCopyInto(out *ISBServiceRolloutStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ISBServiceRolloutStatus.
func (in *ISBServiceRolloutStatus) DeepCopy() *ISBServiceRolloutStatus {
	if in == nil {
		return nil
	}
	out := new(ISBServiceRolloutStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InterStepBufferService) DeepCopyInto(out *InterStepBufferService) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InterStepBufferService.
func (in *InterStepBufferService) DeepCopy() *InterStepBufferService {
	if in == nil {
		return nil
	}
	out := new(InterStepBufferService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KustomizeSource) DeepCopyInto(out *KustomizeSource) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KustomizeSource.
func (in *KustomizeSource) DeepCopy() *KustomizeSource {
	if in == nil {
		return nil
	}
	out := new(KustomizeSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MultiClusterFileGenerator) DeepCopyInto(out *MultiClusterFileGenerator) {
	*out = *in
	if in.Files != nil {
		in, out := &in.Files, &out.Files
		*out = make([]*CredentialedGitLocation, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(CredentialedGitLocation)
				(*in).DeepCopyInto(*out)
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MultiClusterFileGenerator.
func (in *MultiClusterFileGenerator) DeepCopy() *MultiClusterFileGenerator {
	if in == nil {
		return nil
	}
	out := new(MultiClusterFileGenerator)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineRollout) DeepCopyInto(out *PipelineRollout) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineRollout.
func (in *PipelineRollout) DeepCopy() *PipelineRollout {
	if in == nil {
		return nil
	}
	out := new(PipelineRollout)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineRollout) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineRolloutList) DeepCopyInto(out *PipelineRolloutList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PipelineRollout, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineRolloutList.
func (in *PipelineRolloutList) DeepCopy() *PipelineRolloutList {
	if in == nil {
		return nil
	}
	out := new(PipelineRolloutList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineRolloutList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineRolloutSpec) DeepCopyInto(out *PipelineRolloutSpec) {
	*out = *in
	in.Pipeline.DeepCopyInto(&out.Pipeline)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineRolloutSpec.
func (in *PipelineRolloutSpec) DeepCopy() *PipelineRolloutSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineRolloutSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineRolloutStatus) DeepCopyInto(out *PipelineRolloutStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineRolloutStatus.
func (in *PipelineRolloutStatus) DeepCopy() *PipelineRolloutStatus {
	if in == nil {
		return nil
	}
	out := new(PipelineRolloutStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RawSource) DeepCopyInto(out *RawSource) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RawSource.
func (in *RawSource) DeepCopy() *RawSource {
	if in == nil {
		return nil
	}
	out := new(RawSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RepoCredential) DeepCopyInto(out *RepoCredential) {
	*out = *in
	if in.HTTPCredential != nil {
		in, out := &in.HTTPCredential, &out.HTTPCredential
		*out = new(HTTPCredential)
		(*in).DeepCopyInto(*out)
	}
	if in.SSHCredential != nil {
		in, out := &in.SSHCredential, &out.SSHCredential
		*out = new(SSHCredential)
		(*in).DeepCopyInto(*out)
	}
	if in.TLS != nil {
		in, out := &in.TLS, &out.TLS
		*out = new(TLS)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RepoCredential.
func (in *RepoCredential) DeepCopy() *RepoCredential {
	if in == nil {
		return nil
	}
	out := new(RepoCredential)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SSHCredential) DeepCopyInto(out *SSHCredential) {
	*out = *in
	in.SSHKey.DeepCopyInto(&out.SSHKey)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SSHCredential.
func (in *SSHCredential) DeepCopy() *SSHCredential {
	if in == nil {
		return nil
	}
	out := new(SSHCredential)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretKeySelector) DeepCopyInto(out *SecretKeySelector) {
	*out = *in
	out.ObjectReference = in.ObjectReference
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretKeySelector.
func (in *SecretKeySelector) DeepCopy() *SecretKeySelector {
	if in == nil {
		return nil
	}
	out := new(SecretKeySelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretSource) DeepCopyInto(out *SecretSource) {
	*out = *in
	if in.FromKubernetesSecret != nil {
		in, out := &in.FromKubernetesSecret, &out.FromKubernetesSecret
		*out = new(SecretKeySelector)
		**out = **in
	}
	if in.FromFile != nil {
		in, out := &in.FromFile, &out.FromFile
		*out = new(FileKeySelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretSource.
func (in *SecretSource) DeepCopy() *SecretSource {
	if in == nil {
		return nil
	}
	out := new(SecretSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SingleClusterGenerator) DeepCopyInto(out *SingleClusterGenerator) {
	*out = *in
	if in.Values != nil {
		in, out := &in.Values, &out.Values
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SingleClusterGenerator.
func (in *SingleClusterGenerator) DeepCopy() *SingleClusterGenerator {
	if in == nil {
		return nil
	}
	out := new(SingleClusterGenerator)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TLS) DeepCopyInto(out *TLS) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TLS.
func (in *TLS) DeepCopy() *TLS {
	if in == nil {
		return nil
	}
	out := new(TLS)
	in.DeepCopyInto(out)
	return out
}
