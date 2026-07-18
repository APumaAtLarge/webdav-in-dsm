package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
)

type createSymlinkRequest struct {
	Src  string `json:"src"`
	Dist string `json:"dist"`
	Name string `json:"name,omitempty"`
}

type createHardlinkRequest struct {
	Src  string `json:"src"`
	Dist string `json:"dist"`
	Name string `json:"name,omitempty"`
}

type registerUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type createSymlinkResponse struct {
	Link string `json:"link"`
	Src  string `json:"src"`
}

type createHardlinkResponse struct {
	Link string `json:"link"`
	Src  string `json:"src"`
}

type registerUserResponse struct {
	Username string `json:"username"`
	File     string `json:"file"`
}

var htpasswdMu sync.Mutex

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/symlink", handleCreateSymlink)
	mux.HandleFunc("/hardlink", handleCreateHardlink)
	mux.HandleFunc("/register", handleRegisterUser)

	addr := getenv("ADDR", ":8080")
	log.Printf("link api listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleCreateSymlink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	defer r.Body.Close()

	var req createSymlinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	link, src, err := CreateNginxSymlink(req.Src, req.Dist, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, createSymlinkResponse{
		Link: link,
		Src:  src,
	})
}

func handleCreateHardlink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	defer r.Body.Close()

	var req createHardlinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	link, src, err := CreateNginxHardlink(req.Src, req.Dist, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, createHardlinkResponse{
		Link: link,
		Src:  src,
	})
}

func handleRegisterUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	defer r.Body.Close()

	var req registerUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	path := getenv("HTPASSWD_FILE", "/etc/nginx/.htpasswd")
	if err := RegisterHtpasswdUser(path, req.Username, req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, registerUserResponse{
		Username: req.Username,
		File:     path,
	})
}

// CreateNginxSymlink creates an absolute symlink for src under dist.
// Nginx can resolve the link as long as both paths are visible inside the nginx
// runtime environment and the nginx worker user can traverse/read the target.
func CreateNginxSymlink(src, dist, name string) (linkPath string, absSrc string, err error) {
	if strings.TrimSpace(src) == "" {
		return "", "", errors.New("src is required")
	}
	if strings.TrimSpace(dist) == "" {
		return "", "", errors.New("dist is required")
	}

	absSrc, err = filepath.Abs(src)
	if err != nil {
		return "", "", fmt.Errorf("resolve src: %w", err)
	}
	absDist, err := filepath.Abs(dist)
	if err != nil {
		return "", "", fmt.Errorf("resolve dist: %w", err)
	}

	if _, err := os.Stat(absSrc); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("src does not exist: %s", absSrc)
		}
		return "", "", fmt.Errorf("stat src: %w", err)
	}
	if err := os.MkdirAll(absDist, 0o755); err != nil {
		return "", "", fmt.Errorf("create dist: %w", err)
	}

	linkName := name
	if linkName == "" {
		linkName = filepath.Base(absSrc)
	}
	if !isSafeLinkName(linkName) {
		return "", "", errors.New("name must be a single path segment")
	}

	linkPath = filepath.Join(absDist, linkName)
	if existing, err := os.Readlink(linkPath); err == nil {
		existingAbs, err := filepath.Abs(filepath.Join(filepath.Dir(linkPath), existing))
		if err != nil {
			return "", "", fmt.Errorf("resolve existing link: %w", err)
		}
		if existingAbs == absSrc {
			return linkPath, absSrc, nil
		}
		return "", "", fmt.Errorf("link already exists and points to %s", existing)
	}

	if _, err := os.Lstat(linkPath); err == nil {
		return "", "", fmt.Errorf("path already exists and is not a symlink: %s", linkPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("stat link path: %w", err)
	}

	if err := os.Symlink(absSrc, linkPath); err != nil {
		return "", "", fmt.Errorf("create symlink: %w", err)
	}
	return linkPath, absSrc, nil
}

// CreateNginxHardlink creates a hard link for src under dist.
func CreateNginxHardlink(src, dist, name string) (linkPath string, absSrc string, err error) {
	if strings.TrimSpace(src) == "" {
		return "", "", errors.New("src is required")
	}
	if strings.TrimSpace(dist) == "" {
		return "", "", errors.New("dist is required")
	}

	absSrc, err = filepath.Abs(src)
	if err != nil {
		return "", "", fmt.Errorf("resolve src: %w", err)
	}
	absDist, err := filepath.Abs(dist)
	if err != nil {
		return "", "", fmt.Errorf("resolve dist: %w", err)
	}

	srcInfo, err := os.Stat(absSrc)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("src does not exist: %s", absSrc)
		}
		return "", "", fmt.Errorf("stat src: %w", err)
	}
	if srcInfo.IsDir() {
		return "", "", errors.New("src must be a file")
	}
	if err := os.MkdirAll(absDist, 0o755); err != nil {
		return "", "", fmt.Errorf("create dist: %w", err)
	}

	linkName := name
	if linkName == "" {
		linkName = filepath.Base(absSrc)
	}
	if !isSafeLinkName(linkName) {
		return "", "", errors.New("name must be a single path segment")
	}

	linkPath = filepath.Join(absDist, linkName)
	if linkInfo, err := os.Stat(linkPath); err == nil {
		if os.SameFile(srcInfo, linkInfo) {
			return linkPath, absSrc, nil
		}
		return "", "", fmt.Errorf("path already exists and is not a hard link to src: %s", linkPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("stat link path: %w", err)
	}

	if err := os.Link(absSrc, linkPath); err != nil {
		return "", "", fmt.Errorf("create hardlink: %w", err)
	}
	return linkPath, absSrc, nil
}

// RegisterHtpasswdUser adds or updates one user in an nginx auth_basic user file.
func RegisterHtpasswdUser(path, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username is required")
	}
	if password == "" {
		return errors.New("password is required")
	}
	if strings.ContainsAny(username, ":\r\n") {
		return errors.New("username must not contain ':', CR, or LF")
	}
	if strings.ContainsAny(password, "\r\n") {
		return errors.New("password must not contain CR or LF")
	}

	htpasswdMu.Lock()
	defer htpasswdMu.Unlock()

	entries, fileMode, uid, gid, err := readHtpasswd(path)
	if err != nil {
		return err
	}

	entries[username] = nginxSHAHash(password)
	return writeHtpasswd(path, entries, fileMode, uid, gid)
}

func readHtpasswd(path string) (map[string]string, os.FileMode, int, int, error) {
	entries := make(map[string]string)
	mode := os.FileMode(0o640)
	uid := os.Geteuid()
	gid := os.Getegid()

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return entries, mode, uid, gid, nil
		}
		return nil, 0, 0, 0, fmt.Errorf("open htpasswd: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, 0, 0, 0, fmt.Errorf("stat htpasswd: %w", err)
	}
	mode = info.Mode().Perm()
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid = int(stat.Uid)
		gid = int(stat.Gid)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, hash, ok := strings.Cut(line, ":")
		if !ok || name == "" {
			return nil, 0, 0, 0, fmt.Errorf("invalid htpasswd entry: %q", line)
		}
		entries[name] = hash
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, 0, 0, fmt.Errorf("read htpasswd: %w", err)
	}
	return entries, mode, uid, gid, nil
}

func writeHtpasswd(path string, entries map[string]string, mode os.FileMode, uid, gid int) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".htpasswd-*")
	if err != nil {
		return fmt.Errorf("create htpasswd temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	usernames := make([]string, 0, len(entries))
	for username := range entries {
		usernames = append(usernames, username)
	}
	sort.Strings(usernames)

	for _, username := range usernames {
		hash := entries[username]
		if _, err := fmt.Fprintf(tmp, "%s:%s\n", username, hash); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("write htpasswd temp file: %w", err)
		}
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod htpasswd temp file: %w", err)
	}
	if os.Geteuid() == 0 {
		if err := tmp.Chown(uid, gid); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("chown htpasswd temp file: %w", err)
		}
	} else if uid != os.Geteuid() || gid != os.Getegid() {
		_ = tmp.Close()
		return fmt.Errorf("htpasswd ownership is %d:%d but process is %d:%d", uid, gid, os.Geteuid(), os.Getegid())
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close htpasswd temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace htpasswd: %w", err)
	}
	return nil
}

func nginxSHAHash(password string) string {
	sum := sha1.Sum([]byte(password))
	return "{SHA}" + base64.StdEncoding.EncodeToString(sum[:])
}

func isSafeLinkName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
