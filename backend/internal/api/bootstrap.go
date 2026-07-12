package api

import "net/http"

type bootstrapResponse struct {
	APIVersion           string          `json:"api_version"`
	CatalogFormatVersion int             `json:"catalog_format_version"`
	ServerStatus         string          `json:"server_status"`
	Features             map[string]bool `json:"features"`
}

func (r *Router) bootstrap(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, bootstrapResponse{
		APIVersion:           "v1",
		CatalogFormatVersion: 1,
		ServerStatus:         "ok",
		Features: map[string]bool{
			"reviews":        r.store != nil,
			"legacy_auth":    r.users != nil,
			"p2p":            false,
			"signed_catalog": r.cfg.CatalogSigningEnabled,
		},
	})
}
