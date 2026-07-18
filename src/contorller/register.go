package contorller

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
)

type registerUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerUserResponse struct {
	Username string `json:"username"`
	File     string `json:"file"`
}

var htpasswdMu sync.Mutex

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
