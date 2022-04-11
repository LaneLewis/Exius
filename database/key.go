package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
)

type DB struct {
	Conn *pgx.Conn
	Lock sync.Mutex
}

type Endpoint struct {
	GetCount   int
	MaxMkcol   int
	MaxPut     int
	MaxPutSize int64
	MaxGet     int
	MkcolCount int
	Path       string
	PutCount   int
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

type KeySet struct {
	CanCreateChild  bool
	KeyValue        string
	Endpoints       map[string]Endpoint
	InitiateExpire  string
	ExpireDelta     int64
	ExpireStarted   bool
	ExpireStartTime int64
}

func AddKey(keyset KeySet, db *DB) (err error) {
	db.Lock.Lock()
	defer db.Lock.Unlock()
	b, err := json.Marshal(keyset.Endpoints)
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec(context.Background(), `INSERT INTO keys VALUES ($1,$2,$3,$4,$5,$6,$7)`, keyset.CanCreateChild, keyset.KeyValue, b, keyset.InitiateExpire, keyset.ExpireDelta, keyset.ExpireStarted, keyset.ExpireStartTime)
	if err != nil {
		return err
	}
	return nil
}

func GetKey(keyValue string, db *DB) (keySet KeySet, err error) {
	db.Lock.Lock()
	defer db.Lock.Unlock()
	err = db.Conn.QueryRow(context.Background(), "select * from keys where KeyValue=$1;", keyValue).Scan(
		&keySet.CanCreateChild,
		&keySet.KeyValue,
		&keySet.Endpoints,
		&keySet.InitiateExpire,
		&keySet.ExpireDelta,
		&keySet.ExpireStarted,
		&keySet.ExpireStartTime)
	if err != nil {
		return keySet, err
	}
	if keySet.KeyValue == "" {
		return keySet, errors.New("no key found in db")
	}
	return keySet, nil
}

func DeleteKey(keyValue string, db *DB) (err error) {
	db.Lock.Lock()
	defer db.Lock.Unlock()
	_, err = db.Conn.Exec(context.Background(), "DELETE from keys where KeyValue=$1;", keyValue)
	if err != nil {
		return err
	}
	return nil
}
func InitiateConnect(url string, timeOut time.Duration) (conn *pgx.Conn, err error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	timeoutTriggered := time.After(timeOut)
	for {
		select {
		case <-timeoutTriggered:
			return nil, fmt.Errorf("db connection failed")

		case <-ticker.C:
			conn, err = pgx.Connect(context.Background(), url)
			if err == nil {
				return conn, nil
			}
			log.Println("failed to create connection", err)
		}
	}
}
func BuildDB(url string) (*DB, error) {
	conn, err := InitiateConnect(url, 10*time.Minute)
	if err != nil {
		return nil, err
	}
	log.Println("connection created")
	_, err = conn.Exec(context.Background(), `create table if not exists 
	keys(CanCreateChild BOOLEAN,
		KeyValue TEXT,
		Endpoints JSONB,
		InitiateExpire TEXT,
		ExpireDelta BIGINT,
		ExpireStarted BOOLEAN,
		ExpireStartTime BIGINT,
		PRIMARY KEY(KeyValue))`)
	if err != nil {
		return nil, err
	}
	return &DB{
		Conn: conn,
		Lock: sync.Mutex{},
	}, nil
}

func DestroyDB(url string) error {
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		return err
	}
	_, err = conn.Exec(context.Background(), "drop table keys")
	if err != nil {
		return err
	}
	return nil
}

func GetEndpointNames(keyValue string, db *DB) (names []string, err error) {
	db.Lock.Lock()
	defer db.Lock.Unlock()
	err = db.Conn.QueryRow(context.Background(), "select ARRAY(select jsonb_object_keys(Endpoints) from keys where KeyValue=$1);", keyValue).Scan(&names)
	if err != nil {
		return names, err
	}
	return names, nil
}

// gets field and the path, if expired, deletes
func GetBoolFieldAndPath(keyValue string, endpoint string, field string, db *DB) (path string, truth bool, err error) {
	var expireStart int64
	var expireDelta int64
	var expireStarted bool
	db.Lock.Lock()
	err = db.Conn.QueryRow(context.Background(), "select Endpoints -> $1 -> 'Path', Endpoints -> $1 -> $2, ExpireDelta, ExpireStartTime, ExpireStarted from keys where KeyValue=$3", endpoint, field, keyValue).Scan(
		&path,
		&truth,
		&expireDelta,
		&expireStart,
		&expireStarted)
	db.Lock.Unlock()
	if expireStarted && int64(time.Now().UnixMilli())-(expireStart+expireDelta) > 0 {
		err = DeleteKey(keyValue, db)
		if err != nil {
			return path, truth, err
		}
		return path, truth, errors.New("key is expired")
	}
	if err != nil {
		return path, truth, err
	}
	return path, truth, nil
}
func GetPutAndPath(keyValue string, endpoint string, db *DB) (path string, truth bool, putTypes []string, maxPutSize int64, err error) {
	var expireStart int64
	var expireDelta int64
	var maxPut int
	var putCount int
	var expireStarted bool
	db.Lock.Lock()
	err = db.Conn.QueryRow(context.Background(), `
		select Endpoints -> $1 -> 'Path',
		Endpoints -> $1 -> 'Put',
		ExpireDelta,
		ExpireStartTime,
		ExpireStarted,
		Endpoints -> $1 -> 'MaxPut',
		Endpoints -> $1 -> 'PutCount',
		Endpoints -> $1 -> 'PutTypes',
		Endpoints -> $1 -> 'MaxPutSize'
		from keys where KeyValue=$2`, endpoint, keyValue).Scan(
		&path,
		&truth,
		&expireDelta,
		&expireStart,
		&expireStarted,
		&maxPut,
		&putCount,
		&putTypes,
		&maxPutSize,
	)
	db.Lock.Unlock()
	fmt.Println(maxPutSize)
	if err != nil {
		return path, truth, putTypes, maxPutSize, errors.New("error getting rows")
	}
	if expireStarted && int64(time.Now().UnixMilli())-(expireStart+expireDelta) > 0 {
		err = DeleteKey(keyValue, db)
		if err != nil {
			return path, truth, putTypes, maxPutSize, err
		}
		return path, truth, putTypes, maxPutSize, errors.New("key is expired")
	}
	if putCount >= maxPut {
		return path, truth, putTypes, maxPutSize, errors.New("putCount exceeds maxPut")
	}
	return path, truth, putTypes, maxPutSize, nil
}

func GetMkcolAndPath(keyValue string, endpoint string, db *DB) (path string, truth bool, err error) {
	var expireStart int64
	var expireDelta int64
	var maxMkcol int
	var mkcolCount int
	var expireStarted bool
	db.Lock.Lock()
	err = db.Conn.QueryRow(context.Background(), `
		select Endpoints -> $1 -> 'Path',
		Endpoints -> $1 -> 'Mkcol',
		ExpireDelta,
		ExpireStartTime,
		ExpireStarted,
		Endpoints -> $1 -> 'MaxMkcol',
		Endpoints -> $1 -> 'MkcolCount'
		from keys where KeyValue=$2`, endpoint, keyValue).Scan(&path, &truth, &expireDelta, &expireStart, &expireStarted, &maxMkcol, &mkcolCount)
	db.Lock.Unlock()
	if err != nil {
		return path, truth, err
	}
	if expireStarted && int64(time.Now().UnixMilli())-(expireStart+expireDelta) > 0 {
		err = DeleteKey(keyValue, db)
		if err != nil {
			return path, truth, err
		}
		return path, truth, errors.New("key is expired")
	}
	if mkcolCount >= maxMkcol {
		return path, truth, errors.New("mkcolCount exceeds maxMkcol")
	}
	return path, truth, nil
}

func GetAndPath(keyValue string, endpoint string, db *DB) (path string, truth bool, err error) {
	var expireStart int64
	var expireDelta int64
	var maxGet int
	var getCount int
	var expireStarted bool
	db.Lock.Lock()
	err = db.Conn.QueryRow(context.Background(), `
		select Endpoints -> $1 -> 'Path',
		Endpoints -> $1 -> 'Get',
		ExpireDelta,
		ExpireStartTime,
		ExpireStarted,
		Endpoints -> $1 -> 'MaxGet',
		Endpoints -> $1 -> 'GetCount'
		from keys where KeyValue=$2`, endpoint, keyValue).Scan(
		&path,
		&truth,
		&expireDelta,
		&expireStart,
		&expireStarted,
		&maxGet,
		&getCount)
	db.Lock.Unlock()
	if err != nil {
		return path, truth, err
	}
	if expireStarted && int64(time.Now().UnixMilli())-(expireStart+expireDelta) > 0 {
		err = DeleteKey(keyValue, db)
		if err != nil {
			return path, truth, err
		}
		return path, truth, errors.New("key is expired")
	}
	if getCount >= maxGet {
		return path, truth, errors.New("get exceeds maxGet")
	}
	return path, truth, nil
}

func IteratePut(keyValue string, endpoint string, db *DB) error {
	var expireStart int64
	var initiateExpire string
	var putCount int
	var expireStarted bool
	db.Lock.Lock()
	err := db.Conn.QueryRow(context.Background(), `
		select
		ExpireStartTime,
		InitiateExpire,
		ExpireStarted,
		Endpoints -> $2 -> 'PutCount'
		from keys where KeyValue=$1`, keyValue, endpoint).Scan(&expireStart, &initiateExpire, &expireStarted, &putCount)
	db.Lock.Unlock()
	if err != nil {
		return err
	}
	if initiateExpire == "Put" {
		if !expireStarted {
			command := `update keys set expirestarttime=$1, expireStarted=true, Endpoints=jsonb_set(Endpoints, '{` + endpoint + `, PutCount}', ($3::TEXT)::jsonb) where KeyValue=$2;`
			_, err = db.Conn.Exec(context.Background(), command, int64(time.Now().UnixMilli()), keyValue, strconv.Itoa(putCount+1))
			if err != nil {
				return err
			}
			return nil
		}
	} else {
		command := `update keys set Endpoints=jsonb_set(Endpoints, '{` + endpoint + `, PutCount}', ($2::TEXT)::jsonb) where KeyValue=$1;`
		_, err = db.Conn.Exec(context.Background(), command, keyValue, strconv.Itoa(putCount+1))
		if err != nil {
			return err
		}
	}
	return nil
}

func IterateMkcol(keyValue string, endpoint string, db *DB) error {
	var expireStart int64
	var initiateExpire string
	var mkcolCount int
	var expireStarted bool
	db.Lock.Lock()
	err := db.Conn.QueryRow(context.Background(), `
		select
		ExpireStartTime,
		InitiateExpire,
		ExpireStarted,
		Endpoints -> $2 -> 'MkcolCount'
		from keys where KeyValue=$1`, keyValue, endpoint).Scan(&expireStart, &initiateExpire, &expireStarted, &mkcolCount)
	db.Lock.Unlock()
	if err != nil {
		return err
	}
	if initiateExpire == "Mkcol" {
		if !expireStarted {
			command := `update keys set expirestarttime=$1, expireStarted=true, Endpoints=jsonb_set(Endpoints, '{` + endpoint + `, MkcolCount}', ($3::TEXT)::jsonb) where KeyValue=$2;`
			_, err = db.Conn.Exec(context.Background(), command, int64(time.Now().UnixMilli()), keyValue, strconv.Itoa(mkcolCount+1))
			if err != nil {
				return err
			}
			return nil
		}
	} else {
		command := `update keys set Endpoints=jsonb_set(Endpoints, '{` + endpoint + `, MkcolCount}', ($2::TEXT)::jsonb) where KeyValue=$1;`
		_, err = db.Conn.Exec(context.Background(), command, keyValue, strconv.Itoa(mkcolCount+1))
		if err != nil {
			return err
		}
	}

	return nil
}

func IterateGet(keyValue string, endpoint string, db *DB) error {
	var expireStart int64
	var initiateExpire string
	var getCount int
	var expireStarted bool
	db.Lock.Lock()
	err := db.Conn.QueryRow(context.Background(), `
		select
		ExpireStartTime,
		InitiateExpire,
		ExpireStarted,
		Endpoints -> $2 -> 'GetCount'
		from keys where KeyValue=$1`, keyValue, endpoint).Scan(&expireStart, &initiateExpire, &expireStarted, &getCount)
	db.Lock.Unlock()
	if err != nil {
		return err
	}
	if initiateExpire == "Get" {
		if !expireStarted {
			command := `update keys set expirestarttime=$1, expireStarted=true, Endpoints=jsonb_set(Endpoints, '{` + endpoint + `, GetCount}', ($3::TEXT)::jsonb) where KeyValue=$2;`
			_, err = db.Conn.Exec(context.Background(), command, int64(time.Now().UnixMilli()), keyValue, strconv.Itoa(getCount+1))
			if err != nil {
				return err
			}
			return nil
		}
	} else {
		command := `update keys set Endpoints=jsonb_set(Endpoints, '{` + endpoint + `, GetCount}', ($2::TEXT)::jsonb) where KeyValue=$1;`
		_, err = db.Conn.Exec(context.Background(), command, keyValue, strconv.Itoa(getCount+1))
		if err != nil {
			return err
		}

	}
	return nil
}
func ClearExpiredKeys(db *DB) {
	ticker := time.NewTicker(5 * time.Hour)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				db.Lock.Lock()
				_, err := db.Conn.Exec(context.Background(), "DELETE from keys where ExpireStarted=true AND ExpireStartTime+ExpireDelta < $1;", time.Now().UnixMilli())
				db.Lock.Unlock()
				if err != nil {
					fmt.Println("error when deleting expired keys:", err)
				} else {
					fmt.Println("deleted expired keys")
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}
func AddAdmin(adminKey string, db *DB) (err error) {
	baseKey := KeySet{
		CanCreateChild: true,
		KeyValue:       adminKey,
		Endpoints: map[string]Endpoint{
			"root": {
				GetCount:   0,
				MaxMkcol:   2147483647,
				MaxPut:     2147483647,
				MaxPutSize: 9223372036854775807,
				MaxGet:     2147483647,
				MkcolCount: 0,
				Path:       "/",
				PutCount:   0,
				PutTypes:   []string{"any"},
				Copy:       true,
				Delete:     true,
				Get:        true,
				Head:       true,
				Lock:       true,
				Mkcol:      true,
				Options:    true,
				Post:       true,
				Propfind:   true,
				Put:        true,
				Trace:      true,
				Unlock:     true,
			},
		},
		InitiateExpire:  "Never",
		ExpireDelta:     9223372036854775807,
		ExpireStarted:   false,
		ExpireStartTime: 0,
	}
	err = AddKey(baseKey, db)
	if err != nil {
		if strings.Contains(fmt.Sprint(err), "23505") {
			return errors.New("admin key already exists")
		}
		return err
	}
	return nil
}
