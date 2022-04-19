package handles

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/lanelewis/rclone-proxy/database"
)

const proxyURL = "http://localhost:8081"

func serveProxy(target string, path string, method string, key string, endpoint string, db *database.DB, res http.ResponseWriter, req *http.Request) {
	url, _ := url.Parse(target)
	originalURL := fmt.Sprint(req.URL)
	proxy := httputil.NewSingleHostReverseProxy(url)
	req.URL.Host = url.Host
	req.URL.Scheme = url.Scheme
	req.URL.Path = path
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	req.Host = url.Host
	if method == "Propfind" {
		proxy.ModifyResponse = propfindProxyResp(originalURL)
		proxy.ServeHTTP(res, req)
	} else if method == "Put" {
		proxy.ModifyResponse = putProxyResp(key, endpoint, db)
		proxy.ServeHTTP(res, req)
	} else if method == "Get" {
		proxy.ModifyResponse = getProxyResp(key, endpoint, db)
		proxy.ServeHTTP(res, req)
	} else if method == "Mkcol" {
		proxy.ModifyResponse = mkcolProxyResp(key, endpoint, db)
		proxy.ServeHTTP(res, req)
	} else {
		proxy.ServeHTTP(res, req)
	}
	log.Println("reverse-proxy: ", originalURL, " -> ", req.URL)
}

func AuthenticateAndRoute(field string, db *database.DB, w http.ResponseWriter, r *http.Request) (err error) {
	_, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("no basic auth passed")
	}
	origPath := strings.Split(r.URL.Path, "/")[1:]
	if len(origPath) < 2 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("invalid URL")
	}
	var proxyPath string
	var access bool
	var putTypes []string
	var maxPutSize int64
	if field == "Put" {
		proxyPath, access, putTypes, maxPutSize, err = database.GetPutAndPath(password, origPath[1], db)
		if err != nil || !access {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return errors.New("no access to method")
		}
		valid := isFileValid(putTypes, maxPutSize, w, r)
		if !valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return errors.New("invalid file type")
		}
	} else if field == "Mkcol" {
		//not finished implementation
		proxyPath, access, err = database.GetMkcolAndPath(password, origPath[1], db)
	} else if field == "Get" {
		proxyPath, access, err = database.GetAndPath(password, origPath[1], db)
	} else {
		proxyPath, access, err = database.GetBoolFieldAndPath(password, origPath[1], field, db)
	}
	proxyPath = strings.Trim(proxyPath, `"`)
	if err != nil || !access {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("no access to method")
	}
	targetString := proxyURL
	if proxyPath == "/" {
		serveProxy(targetString, strings.Join(origPath[2:], "/"), field, password, origPath[1], db, w, r)
	} else {
		serveProxy(targetString, proxyPath+"/"+strings.Join(origPath[2:], "/"), field, password, origPath[1], db, w, r)
	}
	return nil
}

func propfindProxyResp(originalURL string) func(res *http.Response) error {
	return func(res *http.Response) error {
		context := strings.Split(originalURL, "/")[2]
		b, err := io.ReadAll(res.Body)
		if err != nil {
			log.Fatalln(err)
		}
		res.Body.Close()
		b = bytes.Replace(b, []byte("<D:href>"), []byte(fmt.Sprintf("<D:href>/files/%s", context)), -1)
		body := ioutil.NopCloser(bytes.NewReader(b))
		res.Body = body
		res.ContentLength = int64(len(b))
		res.Header.Set("Content-Length", strconv.Itoa(len(b)))
		return nil
	}
}

func putProxyResp(key string, endpoint string, db *database.DB) func(res *http.Response) error {
	return func(res *http.Response) error {
		if res.StatusCode == 200 || res.StatusCode == 201 {
			err := database.IteratePut(key, endpoint, db)
			if err != nil {
				return err
			}
			return nil
		}
		return errors.New("bad put")
	}
}

func getProxyResp(key string, endpoint string, db *database.DB) func(res *http.Response) error {
	return func(res *http.Response) error {
		if res.StatusCode == 200 || res.StatusCode == 201 {
			err := database.IterateGet(key, endpoint, db)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func mkcolProxyResp(key string, endpoint string, db *database.DB) func(res *http.Response) error {
	return func(res *http.Response) error {
		if res.StatusCode == 200 || res.StatusCode == 201 {
			err := database.IterateMkcol(key, endpoint, db)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func contains(arr []string, value string) (index int, valid bool) {
	for i, val := range arr {
		if val == value {
			return i, true
		}
	}
	return 0, false
}

func isFileValid(fileTypes []string, maxFileSize int64, w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize-1)
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		return false
	}
	_, anyInTypes := contains(fileTypes, "any")
	if anyInTypes {
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		return true
	}
	mimeType := http.DetectContentType(b)
	_, valid := contains(fileTypes, strings.Split(mimeType, ";")[0])
	r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
	return valid
}
