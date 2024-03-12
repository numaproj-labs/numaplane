package kubernetes

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yamlserializer "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/validation"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
)

// validManifestExtensions contains the supported extension for raw file.
var validManifestExtensions = map[string]struct{}{"yaml": {}, "yml": {}, "json": {}}

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
	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}
	if secretName == "" {
		return nil, fmt.Errorf("secretName cannot be empty")
	}
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
func ownerExists(existingRefs []interface{}, ownerRef map[string]interface{}) bool {
	var alreadyExists bool
	for _, ref := range existingRefs {
		if refMap, ok := ref.(map[string]interface{}); ok {
			if refMap["uid"] == ownerRef["uid"] {
				alreadyExists = true
				break
			}
		}
	}
	return alreadyExists
}

func ApplyGitSyncOwnership(manifest string, gitSync *v1alpha1.GitSync) ([]byte, error) {
	// Decode YAML into an Unstructured object
	decUnstructured := yamlserializer.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
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
	alreadyExists := ownerExists(existingRefs, ownerRef)

	// Add the new owner reference if it does not exist
	if !alreadyExists {
		existingRefs = append(existingRefs, ownerRef)
		err = unstructured.SetNestedSlice(obj.Object, existingRefs, "metadata", "ownerReferences")
		if err != nil {
			return nil, err
		}
	}

	// Marshal the updated object into YAML
	modifiedManifest, err := yaml.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return modifiedManifest, nil
}

func IsValidKubernetesManifestFile(fileName string) bool {
	fileExt := strings.Split(fileName, ".")
	if _, ok := validManifestExtensions[fileExt[len(fileExt)-1]]; ok {
		return true
	}
	return false
}
