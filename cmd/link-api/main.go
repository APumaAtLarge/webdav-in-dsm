package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type createSymlinkRequest struct {
	Src  string `json:"src"`
	Dist string `json:"dist"`
	Name string `json:"name,omitempty"`
}

type createSymlinkResponse struct {
	Link string `json:"link"`
	Src  string `json:"src"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/symlink", handleCreateSymlink)

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
