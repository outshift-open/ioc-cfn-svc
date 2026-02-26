package easyhttp

import (
	"encoding/json"
	"net/http"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
)

func GetURLPathUUID(r *http.Request, key string) (string, error) {
	id := r.PathValue(key)
	err := uuid.Validate(id)
	if err != nil {
		return "", errors.Errorf("path param must be valid uuid: %s", err)
	}
	return id, nil
}

func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) (
	int, error) {
	log := getLogger()
	response, err := json.Marshal(payload)
	if err != nil {
		log.Warnf("error marshaling response to json: %s", err)
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
	return code, nil
}

func PathParam(r *http.Request, key string) string {
	return r.PathValue(key)
}

func (rtr *Router) HandleHTTP(path string, h http.Handler) {
	rtr.mux.Handle(path, h)
}
