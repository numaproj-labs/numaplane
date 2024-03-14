package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	serializeryaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
)

func IsValidKubernetesNamespace(name string) bool {
	// All namespace names must be valid RFC 1123 DNS labels.
	errs := validation.IsDNS1123Label(name)
	reservedNamesRegex := regexp.MustCompile(`^(kubernetes-|kube-)`)
	if len(errs) == 0 && !reservedNamesRegex.MatchString(name) {
		return true
	}
	return false
}

// GetGitSyncInstanceAnnotation returns the application instance name from annotation
func GetGitSyncInstanceAnnotation(un *unstructured.Unstructured, key string) (string, error) {
	annotations, err := nestedNullableStringMap(un.Object, "metadata", "annotations")
	if err != nil {
		return "", fmt.Errorf("failed to get annotations from target object %s %s/%s: %w", un.GroupVersionKind().String(), un.GetNamespace(), un.GetName(), err)
	}
	if annotations != nil {
		return annotations[key], nil
	}
	return "", nil
}

// SetGitSyncInstanceAnnotation sets the recommended app.kubernetes.io/instance annotation against an unstructured object
func SetGitSyncInstanceAnnotation(target *unstructured.Unstructured, key, val string) error {
	annotations, err := nestedNullableStringMap(target.Object, "metadata", "annotations")
	if err != nil {
		return fmt.Errorf("failed to get annotations from target object %s %s/%s: %w", target.GroupVersionKind().String(), target.GetNamespace(), target.GetName(), err)
	}

	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = val
	target.SetAnnotations(annotations)
	return nil
}

// nestedNullableStringMap returns a copy of map[string]string value of a nested field.
// Returns an error if not one of map[string]interface{} or nil, or contains non-string values in the map.
func nestedNullableStringMap(obj map[string]interface{}, fields ...string) (map[string]string, error) {
	var m map[string]string
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil {
		return nil, err
	}
	if found && val != nil {
		val, _, err := unstructured.NestedStringMap(obj, fields...)
		return val, err
	}
	return m, err
}

// GetSecret gets secret using the kubernetes client
func GetSecret(ctx context.Context, client k8sClient.Client, namespace, secretName string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	key := k8sClient.ObjectKey{
		Namespace: namespace,
		Name:      secretName,
	}
	if err := client.Get(ctx, key, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// ownerExists checks if an owner reference already exists in the list of owner references.
func ownerExists(existingRefs []metav1.OwnerReference, newRef metav1.OwnerReference) bool {
	for _, ref := range existingRefs {
		if ref.UID == newRef.UID {
			return true
		}
	}
	return false
}

// ApplyGitSyncOwnership2 sets the GitSync as the Owner (or one of the Owners if ones already exist) of the Resource through an OwnerReference
func ApplyGitSyncOwnership2(manifest string, gitSync *v1alpha1.GitSync) ([]byte, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	decoded, _, err := decode([]byte(manifest), nil, nil)
	if err != nil {
		return nil, err
	}
	obj, ok := decoded.(metav1.Object)
	if !ok {
		return nil, errors.New("decoded manifest is not a metaV1 object")
	}
	ownerRef := metav1.OwnerReference{
		APIVersion:         gitSync.APIVersion,
		Kind:               gitSync.Kind,
		Name:               gitSync.Name,
		UID:                gitSync.UID,
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}
	existingRef := obj.GetOwnerReferences()
	if !ownerExists(existingRef, ownerRef) {
		existingRef = append(existingRef, ownerRef)
		obj.SetOwnerReferences(existingRef)
	}
	modifiedManifest, err := yaml.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return modifiedManifest, nil
}

func ApplyGitSyncOwnership(manifest string, gitSync *v1alpha1.GitSync) ([]byte, error) {
	// Decode YAML into an Unstructured object
	decUnstructured := serializeryaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err := decUnstructured.Decode([]byte(manifest), nil, obj)
	if err != nil {
		return nil, err
	}

	// Construct the new owner reference
	ownerRef := map[string]interface{}{
		"apiVersion":         gitSync.APIVersion,
		"kind":               gitSync.Kind,
		"name":               gitSync.Name,
		"uid":                string(gitSync.UID),
		"controller":         true,
		"blockOwnerDeletion": true,
	}

	// Get existing owner references and check if our reference is already there
	existingRefs, found, err := unstructured.NestedSlice(obj.Object, "metadata", "ownerReferences")
	if err != nil {
		return nil, err
	}
	if !found {
		existingRefs = []interface{}{}
	}

	// Check if the owner reference already exists to avoid duplication

	// Add the new owner reference if it does not exist
	if !alreadyExists {
		existingRefs = append(existingRefs, ownerRef)
		err = unstructured.SetNestedSlice(obj.Object, existingRefs, "metadata", "ownerReferences")
		if err != nil {
			return nil, err
		}
	}

	modifiedManifest, err := yaml.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return modifiedManifest, nil
}

func IsValidKubernetesManifestFile(fileName string) bool {
	fileExt := strings.Split(fileName, ".")
	validExtName := []string{"yaml", "yml", "json"}
	return slices.Contains(validExtName, fileExt[len(fileExt)-1])
}
