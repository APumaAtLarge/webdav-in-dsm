package contorller

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

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
