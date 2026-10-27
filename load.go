package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/sandbox-kit-spec/v3/spec"
)

// loadDescriptor reads a Kit descriptor from one of:
//   - authored YAML (examples/*.yaml)
//   - published annotation JSON (YAML-compatible)
//   - an OCI image manifest JSON that carries vnd.docker.sandbox.kit.descriptor
//   - a directory with manifest.json or index.json pointing at such a manifest
func loadDescriptor(path string) (*spec.Descriptor, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	if info.IsDir() {
		for _, name := range []string{"manifest.json", "index.json"} {
			candidate := filepath.Join(path, name)
			if _, err := os.Stat(candidate); err == nil {
				return loadDescriptor(candidate)
			}
		}
		return nil, "", fmt.Errorf("%s: no manifest.json or index.json", path)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	if d, err := spec.Decode(raw); err == nil {
		return d, "descriptor", nil
	}

	var manifest struct {
		SchemaVersion int               `json:"schemaVersion"`
		MediaType     string            `json:"mediaType"`
		Annotations   map[string]string `json:"annotations"`
		Manifests     []struct {
			Digest      string            `json:"digest"`
			MediaType   string            `json:"mediaType"`
			Annotations map[string]string `json:"annotations"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, "", fmt.Errorf("%s: not a kit descriptor and not an OCI manifest: %w", path, err)
	}

	if ann := manifest.Annotations[spec.AnnotationDescriptor]; ann != "" {
		d, err := spec.Decode([]byte(ann))
		if err != nil {
			return nil, "", fmt.Errorf("%s: annotation %s: %w", path, spec.AnnotationDescriptor, err)
		}
		return d, "oci-annotation", nil
	}

	// index.json: follow the first manifest that carries the annotation, or
	// the first digest into blobs/sha256/<hex> beside the index.
	dir := filepath.Dir(path)
	for _, m := range manifest.Manifests {
		if ann := m.Annotations[spec.AnnotationDescriptor]; ann != "" {
			d, err := spec.Decode([]byte(ann))
			if err != nil {
				return nil, "", fmt.Errorf("%s: index annotation: %w", path, err)
			}
			return d, "oci-index-annotation", nil
		}
		if strings.HasPrefix(m.Digest, "sha256:") {
			blob := filepath.Join(dir, "blobs", "sha256", strings.TrimPrefix(m.Digest, "sha256:"))
			if _, err := os.Stat(blob); err == nil {
				return loadDescriptor(blob)
			}
		}
	}

	return nil, "", fmt.Errorf("%s: OCI document has no %s annotation", path, spec.AnnotationDescriptor)
}
