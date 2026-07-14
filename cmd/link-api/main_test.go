package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateNginxSymlink(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()
	src := filepath.Join(root, "source")
	dist := filepath.Join(root, "dist")

	if err := ioutil.WriteFile(src, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	link, absSrc, err := CreateNginxSymlink(src, dist, "")
	if err != nil {
		t.Fatalf("CreateNginxSymlink() error = %v", err)
	}

	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink() error = %v", err)
	}
	if target != absSrc {
		t.Fatalf("link target = %q, want %q", target, absSrc)
	}
}

func TestCreateNginxSymlinkRejectsNestedName(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()
	src := filepath.Join(root, "source")

	if err := ioutil.WriteFile(src, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := CreateNginxSymlink(src, root, "../bad"); err == nil {
		t.Fatal("CreateNginxSymlink() error = nil, want error")
	}
}

func tempDir(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := ioutil.TempDir("", "link-api-test-")
	if err != nil {
		t.Fatal(err)
	}
	return dir, func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}
}
