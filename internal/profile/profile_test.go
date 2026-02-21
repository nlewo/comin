package profile

import (
	"path"
	"slices"
	"testing"

	"os"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestRemoveProfilePath(t *testing.T) {
	dir := t.TempDir()
	file1 := path.Join(dir, "file1")
	_, _ = os.Create(file1)
	file2 := path.Join(dir, "file2")
	_, _ = os.Create(file2)

	_ = RemoveProfilePath(file1)
	entries, _ := os.ReadDir(dir)
	files := make([]string, len(entries))
	for i, e := range entries {
		files[i] = e.Name()
	}
	expected := []string{"file2"}
	assert.Equal(t, expected, files)
}

func TestGarbageCollectProfiles(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	systemProfilePath := t.TempDir()
	_ = os.Mkdir(systemProfilePath, 0700)
	_, _ = os.Create(systemProfilePath + "/comin-5-link")
	_, _ = os.Create(systemProfilePath + "/comin-4-link")
	_, _ = os.Create(systemProfilePath + "/comin-3-link")
	_, _ = os.Create(systemProfilePath + "/comin-2-link")
	_, _ = os.Create(systemProfilePath + "/comin-1-link")
	_ = os.Symlink(systemProfilePath+"/comin-2-link", systemProfilePath+"/comin")

	bootEntries := []string{systemProfilePath + "/comin-3-link", systemProfilePath + "/comin-5-link"}
	removeProfiles(systemProfilePath, "comin", bootEntries)

	dirEntries, _ := os.ReadDir(systemProfilePath)
	entries := []string{}
	for _, d := range dirEntries {
		entries = append(entries, d.Name())
	}

	expected := []string{
		"comin",        // kept because current profile
		"comin-2-link", // kept because current profile target
		"comin-3-link", // kept because tracked by a deployment
		"comin-5-link", // kept because tracked by a deployment
	}
	slices.Sort(entries)
	assert.Equal(t, expected, entries)
}
