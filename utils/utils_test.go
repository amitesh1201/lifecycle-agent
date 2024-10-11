package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift-kni/lifecycle-agent/api/seedreconfig"
	"github.com/stretchr/testify/assert"
)

func TestIsIpv6(t *testing.T) {
	testcases := []struct {
		name     string
		ip       string
		expected bool
	}{
		{
			name:     "ipv6 - true",
			ip:       "2620:52:0:198::10",
			expected: true,
		},
		{
			name:     "ipv6 - false",
			ip:       "192,168.127.10",
			expected: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, IsIpv6(tc.ip), tc.expected)
		})
	}
}

func TestCopyFileIfExists(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError bool
		fileExists    bool
	}{
		{
			name:          "Dest folder doesn't exist",
			expectedError: false,
			fileExists:    true,
		},
		{
			name:          "file exists",
			expectedError: false,
			fileExists:    true,
		},
		{
			name:          "file doesn't exist",
			expectedError: false,
			fileExists:    false,
		},
	}

	for _, tc := range testcases {
		tmpDir := t.TempDir()
		t.Run(tc.name, func(t *testing.T) {
			dst := filepath.Join(tmpDir, "destFolder")
			if !tc.expectedError {
				if err := os.MkdirAll(filepath.Join(tmpDir, "destFolder"), 0o700); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
			source := filepath.Join(tmpDir, "test")
			if tc.fileExists {
				f, err := os.Create(source)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				_ = f.Close()
			}

			err := CopyFileIfExists(source, filepath.Join(dst, "test"))
			assert.Equal(t, err != nil, tc.expectedError)
		})
	}
}

func TestCopyReplaceMirrorRegistry(t *testing.T) {
	image := "quay.io/openshift-kni/lifecycle-agent-operator:4.15.0 "
	testcases := []struct {
		name            string
		seedRegistry    string
		clusterRegistry string
		shouldChange    bool
	}{
		{
			name:            "shouldn't change",
			seedRegistry:    "aaa.io",
			clusterRegistry: "bbb.io",
			shouldChange:    false,
		},
		{
			name:            "should change",
			seedRegistry:    "quay.io",
			clusterRegistry: "bbb.io",
			shouldChange:    true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			newImage, err := ReplaceImageRegistry(image, tc.clusterRegistry, tc.seedRegistry)
			assert.Equal(t, err, nil)
			assert.Equal(t, strings.HasPrefix(newImage, tc.clusterRegistry), tc.shouldChange)
		})
	}
}

func TestLoadGroupedManifestsFromPath(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "staterootB")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create restores directory
	restoreDir := filepath.Join(tmpDir, "manifests")
	if err := os.MkdirAll(restoreDir, 0755); err != nil {
		t.Fatalf("Failed to create restore directory: %v", err)
	}

	// Create two subdirectories for restores
	restoreSubDir1 := filepath.Join(restoreDir, "group1")
	if err := os.Mkdir(restoreSubDir1, 0755); err != nil {
		t.Fatalf("Failed to create restore subdirectory: %v", err)
	}
	restoreSubDir2 := filepath.Join(restoreDir, "group2")
	if err := os.Mkdir(restoreSubDir2, 0755); err != nil {
		t.Fatalf("Failed to create restore subdirectory: %v", err)
	}

	restore1File := filepath.Join(restoreSubDir1, "1_default-restore1.yaml")
	if err := os.WriteFile(restore1File, []byte("apiVersion: velero.io/v1\n"+
		"kind: Restore\n"+
		"metadata:\n"+
		"  name: restore1\n"+
		"spec:\n"+
		"  backupName: backup1\n"), 0644); err != nil {
		t.Fatalf("Failed to create restore file: %v", err)
	}
	restore2File := filepath.Join(restoreSubDir1, "2_default-restore2.yaml")
	if err := os.WriteFile(restore2File, []byte("apiVersion: velero.io/v1\n"+
		"kind: Restore\n"+
		"metadata:\n"+
		"  name: restore2\n"+
		"spec:\n"+
		"  backupName: backup2\n"), 0644); err != nil {
		t.Fatalf("Failed to create restore file: %v", err)
	}
	restore3File := filepath.Join(restoreSubDir2, "1_default-restore3.yaml")
	if err := os.WriteFile(restore3File, []byte("apiVersion: velero.io/v1\n"+
		"kind: Restore\n"+
		"metadata:\n"+
		"  name: restore3\n"+
		"spec:\n"+
		"  backupName: backup3\n"), 0644); err != nil {
		t.Fatalf("Failed to create restore file: %v", err)
	}

	manifests, err := LoadGroupedManifestsFromPath(restoreDir, &logr.Logger{})

	if err != nil {
		t.Fatalf("Failed to load restores: %v", err)
	}

	assert.Equal(t, 2, len(manifests[0]))
	assert.Equal(t, 1, len(manifests[1]))

}

func TestReadSeedReconfigurationFromFile(t *testing.T) {
	type dummySeedReconfiguration struct {
		seedreconfig.SeedReconfiguration
		DummyField string `yaml:"dummy_file, omitempty"`
	}
	tmpDir, err := os.MkdirTemp("", "staterootB")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	data := &dummySeedReconfiguration{
		SeedReconfiguration: seedreconfig.SeedReconfiguration{
			APIVersion:  seedreconfig.SeedReconfigurationVersion,
			BaseDomain:  "test.com",
			ClusterName: "test-cluster",
		},
		// testing that adding a dummy field to the struct doesn't break the unmarshalling
		DummyField: "dummy",
	}
	marshalled, err := json.Marshal(data)
	assert.Equal(t, err, nil)
	// omitting node_labels field
	assert.NotContains(t, string(marshalled), "node_labels")
	// Create seed-reconfiguration.yaml file
	seedReconfigurationFile := filepath.Join(tmpDir, "seed-reconfiguration.yaml")
	if err := os.WriteFile(seedReconfigurationFile, marshalled, 0644); err != nil {
		t.Fatalf("Failed to create seed-reconfiguration.yaml file: %v", err)
	}

	seedReconfig, err := ReadSeedReconfigurationFromFile(seedReconfigurationFile)
	assert.Equal(t, err, nil)
	assert.Equal(t, seedReconfig.APIVersion, seedreconfig.SeedReconfigurationVersion)
	assert.Equal(t, seedReconfig.BaseDomain, "test.com")
	assert.Equal(t, seedReconfig.ClusterName, "test-cluster")
	assert.Equal(t, len(seedReconfig.NodeLabels), 0)
}
