package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"entitymatcher/matcher"
	"entitymatcher/store"
	"github.com/stretchr/testify/require"
)

func TestResolveConnectorFilePathDeniesWhenUnset(t *testing.T) {
	t.Setenv(ConnectorFileRootEnv, "")
	_, err := resolveConnectorFilePath("anything.csv")
	require.Error(t, err)
	require.Contains(t, err.Error(), ConnectorFileRootEnv)
}

func TestResolveConnectorFilePathAcceptsFileInRoot(t *testing.T) {
	root := t.TempDir()
	dataPath := filepath.Join(root, "data.csv")
	require.NoError(t, os.WriteFile(dataPath, []byte("header1,header2\nval1,val2"), 0644))

	t.Setenv(ConnectorFileRootEnv, root)

	// Test absolute path
	resolvedAbs, err := resolveConnectorFilePath(dataPath)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(resolvedAbs, root))

	// Test relative path
	resolvedRel, err := resolveConnectorFilePath("data.csv")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(resolvedRel, root))
}

func TestResolveConnectorFilePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	outsidePath := filepath.Join(filepath.Dir(root), "outside.csv")
	require.NoError(t, os.WriteFile(outsidePath, []byte("content"), 0644))

	t.Setenv(ConnectorFileRootEnv, root)
	_, err := resolveConnectorFilePath("../outside.csv")
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside the permitted directory")
}

func TestResolveConnectorFilePathRejectsAbsoluteOutsideRoot(t *testing.T) {
	root := t.TempDir()
	otherDir := t.TempDir()
	secretPath := filepath.Join(otherDir, "secret.csv")
	require.NoError(t, os.WriteFile(secretPath, []byte("content"), 0644))

	t.Setenv(ConnectorFileRootEnv, root)
	_, err := resolveConnectorFilePath(secretPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside the permitted directory")
}

func TestResolveConnectorFilePathRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	realPath := filepath.Join(outsideDir, "real.csv")
	require.NoError(t, os.WriteFile(realPath, []byte("content"), 0644))

	linkPath := filepath.Join(root, "link.csv")
	err := os.Symlink(realPath, linkPath)
	if err != nil {
		t.Skip("symlinks not supported on this filesystem")
	}

	t.Setenv(ConnectorFileRootEnv, root)
	_, err = resolveConnectorFilePath("link.csv")
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside the permitted directory")
}

func TestResolveConnectorFilePathRejectsSiblingPrefix(t *testing.T) {
	parent := t.TempDir()
	privDir := filepath.Join(parent, "priv")
	privateDir := filepath.Join(parent, "private")
	require.NoError(t, os.MkdirAll(privDir, 0755))
	require.NoError(t, os.MkdirAll(privateDir, 0755))
	secretPath := filepath.Join(privateDir, "secret.csv")
	require.NoError(t, os.WriteFile(secretPath, []byte("content"), 0644))

	t.Setenv(ConnectorFileRootEnv, privDir)
	_, err := resolveConnectorFilePath(secretPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside the permitted directory")
}

func TestResolveConnectorFilePathRejectsDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv(ConnectorFileRootEnv, root)
	_, err := resolveConnectorFilePath(root)
	require.Error(t, err)
	require.Contains(t, err.Error(), "file_path is a directory")
}

func TestIntrospectRejectsPathOutsideRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv(ConnectorFileRootEnv, root)
	server := NewServer(store.NewStore())

	body := map[string]interface{}{
		"type":      matcher.SourceTypeCSV,
		"file_path": "/etc/passwd",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/connector/introspect", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server.HandleIntrospectSchema(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NotContains(t, w.Body.String(), "root:")
}

func TestIntrospectAllowsManualDataWithoutFileRoot(t *testing.T) {
	t.Setenv(ConnectorFileRootEnv, "")
	server := NewServer(store.NewStore())

	body := matcher.ConnectionConfig{
		Type:       matcher.SourceTypeCSV,
		ManualData: []map[string]interface{}{{"name": "a"}},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/connector/introspect", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server.HandleIntrospectSchema(w, req)

	require.NotContains(t, w.Body.String(), "server-side file paths are disabled")
}
