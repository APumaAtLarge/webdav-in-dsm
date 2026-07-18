package contorller

import "net/http"

// NewMux returns the HTTP routes for the link API service.
func NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/symlink", handleCreateSymlink)
	mux.HandleFunc("/hardlink", handleCreateHardlink)
	mux.HandleFunc("/register", handleRegisterUser)
	return mux
}
