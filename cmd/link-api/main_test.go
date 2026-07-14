package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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

func TestCreateNginxSymlinkRejectsMissingSource(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()

	_, _, err := CreateNginxSymlink(filepath.Join(root, "missing"), filepath.Join(root, "dist"), "")
	if err == nil {
		t.Fatal("CreateNginxSymlink() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "src does not exist") {
		t.Fatalf("error = %q, want missing source error", err.Error())
	}
}

func TestCreateNginxSymlinkRejectsDistFile(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()
	src := filepath.Join(root, "source")
	dist := filepath.Join(root, "dist")

	if err := ioutil.WriteFile(src, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(dist, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := CreateNginxSymlink(src, dist, "")
	if err == nil {
		t.Fatal("CreateNginxSymlink() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "create dist") {
		t.Fatalf("error = %q, want dist creation error", err.Error())
	}
}

func TestCreateNginxSymlinkRejectsExistingNonSymlink(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()
	src := filepath.Join(root, "source")
	dist := filepath.Join(root, "dist")
	link := filepath.Join(dist, "source")

	if err := os.MkdirAll(dist, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(src, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(link, []byte("already here"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := CreateNginxSymlink(src, dist, "")
	if err == nil {
		t.Fatal("CreateNginxSymlink() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "not a symlink") {
		t.Fatalf("error = %q, want non-symlink conflict error", err.Error())
	}
}

func TestCreateNginxSymlinkRejectsUnreadableSourcePath(t *testing.T) {
	skipIfRoot(t)

	root, cleanup := tempDir(t)
	defer cleanup()
	locked := filepath.Join(root, "locked")
	src := filepath.Join(locked, "source")
	dist := filepath.Join(root, "dist")

	if err := os.MkdirAll(locked, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(src, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0755)

	_, _, err := CreateNginxSymlink(src, dist, "")
	if err == nil {
		t.Fatal("CreateNginxSymlink() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "stat src") {
		t.Fatalf("error = %q, want source stat permission error", err.Error())
	}
}

func TestCreateNginxSymlinkRejectsUnwritableDist(t *testing.T) {
	skipIfRoot(t)

	root, cleanup := tempDir(t)
	defer cleanup()
	src := filepath.Join(root, "source")
	dist := filepath.Join(root, "dist")

	if err := ioutil.WriteFile(src, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dist, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dist, 0755)

	_, _, err := CreateNginxSymlink(src, dist, "")
	if err == nil {
		t.Fatal("CreateNginxSymlink() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "create symlink") {
		t.Fatalf("error = %q, want symlink permission error", err.Error())
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

func skipIfRoot(t *testing.T) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("permission checks are not reliable when running as root")
	}
}
