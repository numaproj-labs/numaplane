package kubernates

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GetGitSyncInstanceAnnotation returns the application instance name from annotation
func GetGitSyncInstanceAnnotation(un *unstructured.Unstructured, key string) (string, error) {
	annotations, _, err := nestedNullableStringMap(un.Object, "metadata", "annotations")
	if err != nil {
		return "", fmt.Errorf("failed to get annotations from target object %s %s/%s: %w", un.GroupVersionKind().String(), un.GetNamespace(), un.GetName(), err)
	}
	if annotations != nil {
		return annotations[key], nil
	}
	return "", nil
}

// SetGitSyncInstanceAnnotation the recommended app.kubernetes.io/instance annotation against an unstructured object
// Uses the legacy labeling if environment variable is set
func SetGitSyncInstanceAnnotation(target *unstructured.Unstructured, key, val string) error {
	annotations, _, err := nestedNullableStringMap(target.Object, "metadata", "annotations")
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
// Returns false if value is not found and an error if not one of map[string]interface{} or nil, or contains non-string values in the map.
func nestedNullableStringMap(obj map[string]interface{}, fields ...string) (map[string]string, bool, error) {
	var m map[string]string
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil {
		return nil, found, err
	}
	if found && val != nil {
		return unstructured.NestedStringMap(obj, fields...)
	}
	return m, found, err
}
