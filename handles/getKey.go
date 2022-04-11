package handles

import (
	"encoding/json"
	"errors"
	"net/http"

	_ "github.com/go-playground/validator/v10"
	"github.com/lanelewis/rclone-proxy/database"
)

func GetKeyHandle(db *database.DB, w http.ResponseWriter, r *http.Request) (err error) {
	_, key, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("no authorization passed")
	}
	keySet, err := database.GetKey(key, db)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("invalid key")
	}
	for k, endpoint := range keySet.Endpoints {
		endpoint.Path = "/"
		keySet.Endpoints[k] = endpoint
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(keySet)
	return nil
}
