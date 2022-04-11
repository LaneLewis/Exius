package handles

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/lanelewis/rclone-proxy/database"
)

const adminURL = "http://localhost:8082"

func AdminHandle(db *database.DB, w http.ResponseWriter, r *http.Request) (err error) {
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
	if !(keySet.KeyValue == os.Getenv("ADMINKEY")) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("invalid key")
	}
	adminProxy(w, r)
	return nil
}
func adminProxy(res http.ResponseWriter, req *http.Request) {
	url, _ := url.Parse(adminURL)
	originalURL := fmt.Sprint(req.URL)
	proxy := httputil.NewSingleHostReverseProxy(url)
	req.URL.Host = url.Host
	req.URL.Scheme = url.Scheme
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	req.Host = url.Host
	fmt.Println("reverse-proxy: ", originalURL, " -> ", req.URL)
	proxy.ServeHTTP(res, req)
}
