package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chdirTemp runs the test in an empty temporary directory (t.Chdir needs Go 1.24)
func chdirTemp(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(t.TempDir()))
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func TestWritePromptLog_IsPrivateToTheOwner(t *testing.T) {
	chdirTemp(t)

	// A log left by an older version is world-readable
	require.NoError(t, os.WriteFile(PromptLogFile, []byte("old"), 0o644))
	require.NoError(t, os.Chmod(PromptLogFile, 0o644))

	require.NoError(t, WritePromptLog("names, bios and messages"))

	content, err := os.ReadFile(PromptLogFile)
	require.NoError(t, err)
	assert.Equal(t, "names, bios and messages", string(content))

	info, err := os.Stat(PromptLogFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWritePromptLog_Error(t *testing.T) {
	chdirTemp(t)
	// A non-empty directory with the log file name cannot be replaced
	require.NoError(t, os.Mkdir(PromptLogFile, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(PromptLogFile, "keep"), nil, 0o600))

	assert.Error(t, WritePromptLog("prompt"))
}

func TestWritePromptLog_DoesNotFollowSymlinks(t *testing.T) {
	chdirTemp(t)
	target := filepath.Join(t.TempDir(), "elsewhere.txt")
	require.NoError(t, os.WriteFile(target, []byte("untouched"), 0o644))
	require.NoError(t, os.Symlink(target, PromptLogFile))

	require.NoError(t, WritePromptLog("secret prompt"))

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "untouched", string(content), "the symlink target is not overwritten")

	info, err := os.Lstat(PromptLogFile)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "the symlink is replaced by a regular file")
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
