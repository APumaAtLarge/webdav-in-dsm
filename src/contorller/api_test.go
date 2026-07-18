package contorller

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

func TestCreateNginxHardlink(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()
	src := filepath.Join(root, "source")
	dist := filepath.Join(root, "dist")

	if err := ioutil.WriteFile(src, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	link, absSrc, err := CreateNginxHardlink(src, dist, "")
	if err != nil {
		t.Fatalf("CreateNginxHardlink() error = %v", err)
	}
	if absSrc != src {
		t.Fatalf("absSrc = %q, want %q", absSrc, src)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	linkInfo, err := os.Stat(link)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(srcInfo, linkInfo) {
		t.Fatal("created path is not a hard link to source")
	}
}

func TestCreateNginxHardlinkIsIdempotentForSameSource(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()
	src := filepath.Join(root, "source")
	dist := filepath.Join(root, "dist")

	if err := ioutil.WriteFile(src, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	first, _, err := CreateNginxHardlink(src, dist, "")
	if err != nil {
		t.Fatalf("CreateNginxHardlink() first error = %v", err)
	}
	second, _, err := CreateNginxHardlink(src, dist, "")
	if err != nil {
		t.Fatalf("CreateNginxHardlink() second error = %v", err)
	}
	if second != first {
		t.Fatalf("second link = %q, want %q", second, first)
	}
}

func TestCreateNginxHardlinkRejectsNestedName(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()
	src := filepath.Join(root, "source")

	if err := ioutil.WriteFile(src, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := CreateNginxHardlink(src, root, "../bad"); err == nil {
		t.Fatal("CreateNginxHardlink() error = nil, want error")
	}
}

func TestCreateNginxHardlinkRejectsDirectorySource(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()
	src := filepath.Join(root, "source")

	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}

	_, _, err := CreateNginxHardlink(src, filepath.Join(root, "dist"), "")
	if err == nil {
		t.Fatal("CreateNginxHardlink() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "src must be a file") {
		t.Fatalf("error = %q, want file source error", err.Error())
	}
}

func TestCreateNginxHardlinkRejectsExistingDifferentFile(t *testing.T) {
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
	if err := ioutil.WriteFile(link, []byte("different"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := CreateNginxHardlink(src, dist, "")
	if err == nil {
		t.Fatal("CreateNginxHardlink() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "not a hard link") {
		t.Fatalf("error = %q, want hard link conflict error", err.Error())
	}
}

func TestRegisterHtpasswdUserCreatesFile(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()
	path := filepath.Join(root, ".htpasswd")

	if err := RegisterHtpasswdUser(path, "alice", "secret"); err != nil {
		t.Fatalf("RegisterHtpasswdUser() error = %v", err)
	}

	content := readFile(t, path)
	if !strings.Contains(content, "alice:{SHA}") {
		t.Fatalf("content = %q, want alice SHA entry", content)
	}
	if strings.Contains(content, "secret") {
		t.Fatal("htpasswd file contains plaintext password")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0640 {
		t.Fatalf("mode = %o, want 640", got)
	}
}

func TestRegisterHtpasswdUserUpdatesExistingUser(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()
	path := filepath.Join(root, ".htpasswd")

	if err := ioutil.WriteFile(path, []byte("alice:old\nbob:{SHA}Ys23Ag/5IOWqZCw9QGaVDdHwH00=\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RegisterHtpasswdUser(path, "alice", "new-secret"); err != nil {
		t.Fatalf("RegisterHtpasswdUser() error = %v", err)
	}

	content := readFile(t, path)
	if strings.Count(content, "alice:") != 1 {
		t.Fatalf("content = %q, want one alice entry", content)
	}
	if !strings.Contains(content, "bob:{SHA}") {
		t.Fatalf("content = %q, want existing bob entry", content)
	}
	if strings.Contains(content, "alice:old") {
		t.Fatalf("content = %q, old alice hash was not replaced", content)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestRegisterHtpasswdUserRejectsInvalidUsername(t *testing.T) {
	root, cleanup := tempDir(t)
	defer cleanup()

	err := RegisterHtpasswdUser(filepath.Join(root, ".htpasswd"), "bad:user", "secret")
	if err == nil {
		t.Fatal("RegisterHtpasswdUser() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "username") {
		t.Fatalf("error = %q, want username error", err.Error())
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

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func skipIfRoot(t *testing.T) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("permission checks are not reliable when running as root")
	}
}
