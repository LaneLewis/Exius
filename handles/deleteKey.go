package handles

import (
	"errors"
	"net/http"

	"github.com/lanelewis/rclone-proxy/database"
)

func DeleteKeyHandle(db *database.DB, w http.ResponseWriter, r *http.Request) (err error) {
	_, keyValue, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("no authorization passed")
	}
	err = database.DeleteKey(keyValue, db)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("invalid key")
	}
	return nil
}
