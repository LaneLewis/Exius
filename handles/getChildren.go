package handles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/lanelewis/rclone-proxy/database"
)

func isProtocolValid(childEndpoint database.Endpoint, parentEndpoint database.Endpoint) bool {
	if validChildFieldPermission(parentEndpoint.Copy, childEndpoint.Copy) &&
		validChildFieldPermission(parentEndpoint.Delete, childEndpoint.Delete) &&
		validChildFieldPermission(parentEndpoint.Get, childEndpoint.Get) &&
		validChildFieldPermission(parentEndpoint.Head, childEndpoint.Head) &&
		validChildFieldPermission(parentEndpoint.Lock, childEndpoint.Lock) &&
		validChildFieldPermission(parentEndpoint.Mkcol, childEndpoint.Mkcol) &&
		validChildFieldPermission(parentEndpoint.Options, childEndpoint.Options) &&
		validChildFieldPermission(parentEndpoint.Propfind, childEndpoint.Propfind) &&
		validChildFieldPermission(parentEndpoint.Put, childEndpoint.Put) &&
		validChildFieldPermission(parentEndpoint.Trace, childEndpoint.Trace) &&
		validChildFieldPermission(parentEndpoint.Unlock, childEndpoint.Unlock) {
		return true
	}
	return false
}

func isPathSubset(parentPath string, childPath string) (subPath string, truth bool) {
	if parentPath == "/" && childPath != "/" {
		return childPath, true
	}
	splitParent := strings.Split(parentPath, "/")
	splitChild := strings.Split(childPath, "/")
	if len(splitParent) > len(splitChild) {
		return "", false
	}
	var i int
	for i = range splitParent {
		if splitParent[i] != splitChild[i] {
			return "", false
		}
	}
	if len(splitParent) == len(splitChild) {
		return "", true
	}
	return strings.Join(splitChild[i:], "/"), true
}

func keyPathMap(keySet database.KeySet) map[string]string {
	keyMap := make(map[string]string)
	for k, endpoint := range keySet.Endpoints {
		keyMap[k] = endpoint.Path
	}
	return keyMap
}

func isEndpointSubset(childEndpoint database.Endpoint, parentMap map[string]database.Endpoint) (newPath string, truth bool) {
	for k, endpoint := range parentMap {
		subPath, isPathSub := isPathSubset(endpoint.Path, childEndpoint.Path)
		if !isPathSub {
			continue
		}
		_, isParentAllType := contains(endpoint.PutTypes, "any")
		if !isParentAllType {
			if !IsArraySubset(endpoint.PutTypes, endpoint.PutTypes) {
				continue
			}
		}
		if endpoint.MaxMkcol < childEndpoint.MaxMkcol {
			continue
		}
		if endpoint.MaxPut < childEndpoint.MaxPut {
			continue
		}
		if !isProtocolValid(childEndpoint, endpoint) {
			continue
		}
		return k + subPath, true
	}
	return "", false
}

type PathObj struct {
	Name string
	Path string
}

func isKeyChild(childKey database.KeySet, parentKey database.KeySet) (truth bool, pathObjs []PathObj) {
	if childKey.ExpireDelta > parentKey.ExpireDelta {
		return false, pathObjs
	}
	if !validChildFieldPermission(parentKey.CanCreateChild, childKey.CanCreateChild) {
		return false, pathObjs
	}
	for cKey, cEndpoint := range childKey.Endpoints {
		newPath, isChild := isEndpointSubset(cEndpoint, parentKey.Endpoints)
		if !isChild {
			return false, pathObjs
		}
		pathObjs = append(pathObjs, PathObj{Name: cKey, Path: newPath})
	}
	return true, pathObjs
}

func IterateDB(parentKey database.KeySet, db *database.DB) (err error, keyMap map[string][]PathObj) {
	err = database.PingReconnect(db)
	if err != nil {
		return err, keyMap
	}
	db.Lock.Lock()
	rows, err := db.Conn.Query(context.Background(), "SELECT * FROM keys")
	if err != nil {
		return err, keyMap
	}
	defer rows.Close()
	keyMap = make(map[string][]PathObj)
	for rows.Next() {
		var keySet database.KeySet
		err := rows.Scan(
			&keySet.CanCreateChild,
			&keySet.KeyValue,
			&keySet.Endpoints,
			&keySet.InitiateExpire,
			&keySet.ExpireDelta,
			&keySet.ExpireStarted,
			&keySet.ExpireStartTime)
		if err != nil {
			return err, keyMap
		}
		isChild, pathObjs := isKeyChild(keySet, parentKey)
		if isChild {
			keyMap[keySet.KeyValue] = pathObjs
		}
	}
	db.Lock.Unlock()
	return err, keyMap
}

func GetChildrenHandle(db *database.DB, w http.ResponseWriter, r *http.Request) (err error) {
	_, keyValue, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("no authorization passed")
	}
	parentKey, err := database.GetKey(keyValue, db)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("invalid key")
	}
	err, keyMap := IterateDB(parentKey, db)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("invalid key")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(keyMap)
	return nil
}
