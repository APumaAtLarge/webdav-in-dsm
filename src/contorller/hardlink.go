package contorller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type createHardlinkRequest struct {
	Src  string `json:"src"`
	Dist string `json:"dist"`
	Name string `json:"name,omitempty"`
}

type createHardlinkResponse struct {
	Link string `json:"link"`
	Src  string `json:"src"`
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
