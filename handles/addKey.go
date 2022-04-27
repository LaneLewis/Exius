package handles

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lanelewis/rclone-proxy/database"

	_ "github.com/go-playground/validator/v10"
	"github.com/sethvargo/go-password/password"
)

//file size limit and addition of read and mkcol
type ClientJson struct {
	CanCreateChild bool
	KeyValue       string
	Endpoints      map[string]json.RawMessage
	InitiateExpire string
	ExpireDelta    uint64
}
type ClientKeySet struct {
	CanCreateChild bool
	KeyValue       string
	Endpoints      map[string]ClientEndpoint
	InitiateExpire string
	ExpireDelta    uint64
}
type ClientEndpoint struct {
	MaxMkcol   uint
	MaxPut     uint
	MaxPutSize int64
	MaxGet     uint
	Path       string
	PutTypes   []string

	Copy     bool
	Delete   bool
	Get      bool
	Head     bool
	Lock     bool
	Mkcol    bool
	Options  bool
	Post     bool
	Propfind bool
	Put      bool
	Trace    bool
	Unlock   bool
}

func getClientEndpoint(keyValue string, db *database.DB) (clientKey ClientKeySet, err error) {
	key, err := database.GetKey(keyValue, db)
	if err != nil {
		return clientKey, err
	}
	clientKeyMap := make(map[string]ClientEndpoint)
	for k, endpoint := range key.Endpoints {
		clientEndpoint := ClientEndpoint{
			MaxMkcol:   uint(endpoint.MaxMkcol),
			MaxPut:     uint(endpoint.MaxPut),
			MaxPutSize: int64(endpoint.MaxPutSize),
			MaxGet:     uint(endpoint.MaxGet),
			Path:       endpoint.Path,
			PutTypes:   endpoint.PutTypes,
			Copy:       endpoint.Copy,
			Delete:     endpoint.Delete,
			Get:        endpoint.Get,
			Head:       endpoint.Head,
			Lock:       endpoint.Lock,
			Mkcol:      endpoint.Mkcol,
			Options:    endpoint.Options,
			Post:       endpoint.Post,
			Put:        endpoint.Put,
			Trace:      endpoint.Trace,
			Unlock:     endpoint.Unlock,
		}
		clientKeyMap[k] = clientEndpoint
	}
	clientKey = ClientKeySet{
		CanCreateChild: key.CanCreateChild,
		KeyValue:       key.KeyValue,
		Endpoints:      clientKeyMap,
		InitiateExpire: key.InitiateExpire,
		ExpireDelta:    uint64(key.ExpireDelta),
	}
	return clientKey, nil
}

func AddKeyHandle(db *database.DB, w http.ResponseWriter, r *http.Request) (err error) {
	_, key, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("no authorization passed")
	}
	parentClientKeySet, err := getClientEndpoint(key, db)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("invalid parent key")
	}
	childClientKeySet, err := parseClientJson(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return errors.New("invalid child json")
	}
	childKeySet, err := ValidateChildKey(childClientKeySet, parentClientKeySet)
	if err != nil {
		http.Error(w, fmt.Sprint("Invalid json body: ", err), http.StatusBadRequest)
		return errors.New("invalid child parameters")
	}
	err = database.AddKey(childKeySet, db)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return errors.New("unable to add key to database")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	// obscure absolute path field for user
	for k, endpoint := range childKeySet.Endpoints {
		endpoint.Path = k
		childKeySet.Endpoints[k] = endpoint
	}
	json.NewEncoder(w).Encode(childKeySet)
	return nil
}

func parseClientJson(r *http.Request) (keyset ClientKeySet, err error) {
	defaultClientJson := ClientJson{CanCreateChild: false, KeyValue: "", Endpoints: make(map[string]json.RawMessage), InitiateExpire: "Creation", ExpireDelta: uint64(time.Hour / time.Millisecond)}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err = dec.Decode(&defaultClientJson)
	if err != nil {
		return keyset, err
	}
	_, validInitiateExpire := contains([]string{"Get", "Put", "Mkcol", "Creation", "Never"}, defaultClientJson.InitiateExpire)
	if !validInitiateExpire {
		return keyset, errors.New("invalid value for initiate expire")
	}
	clientKeySet := ClientKeySet{CanCreateChild: defaultClientJson.CanCreateChild, KeyValue: defaultClientJson.KeyValue, Endpoints: make(map[string]ClientEndpoint), InitiateExpire: defaultClientJson.InitiateExpire, ExpireDelta: defaultClientJson.ExpireDelta}
	for k, v := range defaultClientJson.Endpoints {
		defaultEndpoint := ClientEndpoint{
			MaxMkcol:   2147483647,
			MaxPut:     2147483647,
			MaxPutSize: 9223372036854775807,
			MaxGet:     2147483647,
			Path:       "",
			PutTypes:   []string{"any"},
			Copy:       false,
			Delete:     false,
			Get:        false,
			Head:       false,
			Lock:       false,
			Mkcol:      false,
			Options:    false,
			Post:       false,
			Propfind:   false,
			Put:        false,
			Trace:      false,
			Unlock:     false}
		err = json.Unmarshal(v, &defaultEndpoint)
		if err != nil {
			return keyset, err
		}
		possibleTypes := []string{"any", "application/octet-stream", "application/pdf",
			"application/postscript", "text/plain", "image/x-icon", "image/bmp",
			"image/gif", "image/webp", "image/png", "image/jpeg", "audio/basic",
			"audio/aiff", "audio/mpeg", "application/ogg", "audio/midi", "video/avi",
			"audio/wave", "video/webm", "application/vnd.ms-fontobject", "font/ttf",
			"font/otf", "font/collection", "font/woff", "font/woff2", "application/x-gzip",
			"application/zip", "application/x-rar-compressed", "application/x-rar-compressed",
			"application/wasm", "text/html", "video/mp4"}
		if !IsArraySubset(possibleTypes, defaultEndpoint.PutTypes) {
			return keyset, errors.New("invalid put type")
		}
		clientKeySet.Endpoints[k] = defaultEndpoint
	}
	return clientKeySet, nil
}

func IsArraySubset(parentArr []string, childArr []string) bool {
	for _, arrStr := range childArr {
		_, valid := contains(parentArr, arrStr)
		if !valid {
			return false
		}
	}
	return true
}

func validChildFieldPermission(parentField bool, childField bool) bool {
	if !parentField && childField {
		return false
	}
	return true
}

func getMapKeys(m map[string]ClientEndpoint) []string {
	keys := make([]string, 0)
	for k, _ := range m {
		keys = append(keys, k)
	}
	return keys
}

func areProtocolsValid(childEndpoint ClientEndpoint, parentEndpoint ClientEndpoint) bool {
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

func ValidateChildKey(childKey ClientKeySet, parentKey ClientKeySet) (validKey database.KeySet, err error) {
	if childKey.ExpireDelta > parentKey.ExpireDelta {
		return validKey, errors.New("timeDelta of child exceeds parent")
	}
	if !parentKey.CanCreateChild{
		return validKey, errors.New("parent key does not have the ability to create children")
	}
	validKeyMap := make(map[string]database.Endpoint)
	parentKeyNames := getMapKeys(parentKey.Endpoints)
	for k, endpoint := range childKey.Endpoints {
		childPathArr := strings.Split(strings.Trim(endpoint.Path, "/"), "/")
		_, isChildEndpoint := contains(parentKeyNames, childPathArr[0])
		if !isChildEndpoint {
			return validKey, errors.New("child key endpoint not in parent")
		}
		parentKeyEndpoint := parentKey.Endpoints[childPathArr[0]]
		absoluteChildPath := parentKeyEndpoint.Path + strings.Join(childPathArr[1:], "/")
		_, isParentAllType := contains(parentKeyEndpoint.PutTypes, "any")
		if !isParentAllType {
			if IsArraySubset(parentKeyEndpoint.PutTypes, endpoint.PutTypes) {
				return validKey, errors.New("child key put types not in parent")
			}
		}
		childTypes := endpoint.PutTypes
		_, isChildAllType := contains(endpoint.PutTypes, "any")
		if isChildAllType {
			childTypes = []string{"any"}
		}
		if endpoint.MaxGet > parentKeyEndpoint.MaxGet {
			return validKey, errors.New("child key maxGet exceeds parent maxGet")
		}
		if endpoint.MaxPutSize > parentKeyEndpoint.MaxPutSize {
			return validKey, errors.New("child key maxPutSize exceeds parent maxPutSize")
		}
		if endpoint.MaxMkcol > parentKeyEndpoint.MaxMkcol {
			return validKey, errors.New("child key maxMkcol exceeds parent maxMkcol")
		}
		if endpoint.MaxPut > parentKeyEndpoint.MaxPut {
			return validKey, errors.New("child key maxPut exceeds parent maxPut")
		}
		if !areProtocolsValid(endpoint, parentKeyEndpoint) {
			return validKey, errors.New("child key has protocols that exceed parent")
		}
		validEndpoint := database.Endpoint{
			MaxMkcol:   int(endpoint.MaxMkcol),
			MaxPut:     int(endpoint.MaxPut),
			MaxPutSize: int64(endpoint.MaxPutSize),
			MaxGet:     int(endpoint.MaxGet),
			MkcolCount: 0,
			Path:       absoluteChildPath,
			PutCount:   0,
			PutTypes:   childTypes,
			Copy:       endpoint.Copy,
			Delete:     endpoint.Delete,
			Get:        endpoint.Get,
			Head:       endpoint.Head,
			Lock:       endpoint.Lock,
			Mkcol:      endpoint.Mkcol,
			Options:    endpoint.Options,
			Propfind:   endpoint.Propfind,
			Put:        endpoint.Put,
			Trace:      endpoint.Trace,
			Unlock:     endpoint.Unlock,
		}
		validKeyMap[k] = validEndpoint
	}
	childKeyValue, err := password.Generate(64, 10, 0, false, true)
	if err != nil {
		return validKey, errors.New("key could not be generated")
	}
	validKey = database.KeySet{
		CanCreateChild:  childKey.CanCreateChild,
		KeyValue:        childKeyValue,
		Endpoints:       validKeyMap,
		InitiateExpire:  childKey.InitiateExpire,
		ExpireDelta:     int64(childKey.ExpireDelta),
		ExpireStarted:   false,
		ExpireStartTime: 0,
	}
	if childKey.InitiateExpire == "Creation" {
		validKey.ExpireStartTime = time.Now().UnixMilli()
		validKey.ExpireStarted = true
	} else if childKey.InitiateExpire == "Never" {
		validKey.ExpireDelta = 9223372036854775807
	}
	return validKey, nil
}
