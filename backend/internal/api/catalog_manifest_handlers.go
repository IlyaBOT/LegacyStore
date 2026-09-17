package api

import "net/http"

func (r *Router) catalogManifest(w http.ResponseWriter, req *http.Request) {
	if !r.requireStore(w) {
		return
	}
	if r.signer == nil {
		writeError(w, http.StatusServiceUnavailable, "catalog_signing_unavailable")
		return
	}
	payload, err := r.store.Manifest(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "catalog_manifest_failed")
		return
	}
	envelope, err := r.signer.SignJSON(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "catalog_signing_failed")
		return
	}
	writeJSON(w, http.StatusOK, envelope)
}

func (r *Router) catalogPublicKey(w http.ResponseWriter, req *http.Request) {
	if r.signer == nil {
		writeError(w, http.StatusServiceUnavailable, "catalog_signing_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, r.signer.PublicKeyInfo())
}
