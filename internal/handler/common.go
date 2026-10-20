package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"W-dev/internal/service"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[handler] encode json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeServiceError memetakan error service ke status HTTP.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrSlugTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrQueueFull):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		log.Printf("[handler] error internal: %v", err)
		writeError(w, http.StatusInternalServerError, "terjadi kesalahan internal")
	}
}

// pathID membaca {id} dari URL.
func pathID(r *http.Request) (uint, bool) {
	n, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || n == 0 {
		return 0, false
	}
	return uint(n), true
}
