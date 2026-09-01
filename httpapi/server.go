package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"vadadb/db"
)

type Backend interface {
	db.DB
	CrashShard(int) error
	RecoverShard(int) error
	Snapshot() error
	Shards() []db.ShardInfo
	StorageMetrics() db.StorageMetrics
}

func New(database Backend) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/kv/", func(w http.ResponseWriter, r *http.Request) {
		key, err := parseUintPath(r.URL.Path, "/kv/")
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		switch r.Method {
		case http.MethodPut:
			var input struct {
				Value *uint64 `json:"value"`
			}
			if err := decodeJSON(w, r, &input); err != nil || input.Value == nil {
				if err == nil {
					err = errors.New("value is required")
				}
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if err := database.Put(key, *input.Value); err != nil {
				writeDatabaseError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, db.Record{Key: key, Value: *input.Value})
		case http.MethodGet:
			value, found, err := database.Get(key)
			if err != nil {
				writeDatabaseError(w, err)
				return
			}
			if !found {
				writeError(w, http.StatusNotFound, errors.New("key not found"))
				return
			}
			writeJSON(w, http.StatusOK, db.Record{Key: key, Value: value})
		case http.MethodDelete:
			if err := database.Delete(key); err != nil {
				writeDatabaseError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Allow", "GET, PUT, DELETE")
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		}
	})
	mux.HandleFunc("/scan", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		records, err := database.Scan()
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		if records == nil {
			records = []db.Record{}
		}
		writeJSON(w, http.StatusOK, records)
	})
	mux.HandleFunc("/admin/shards", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, database.Shards())
	})
	mux.HandleFunc("/admin/shards/", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/admin/shards/")
		parts := strings.Split(rest, "/")
		if len(parts) != 2 || parts[0] == "" {
			writeError(w, http.StatusNotFound, errors.New("not found"))
			return
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil || id < 0 {
			writeError(w, http.StatusBadRequest, errors.New("invalid shard id"))
			return
		}
		switch parts[1] {
		case "crash":
			err = database.CrashShard(id)
		case "recover":
			err = database.RecoverShard(id)
		default:
			writeError(w, http.StatusNotFound, errors.New("not found"))
			return
		}
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, database.Shards()[id])
	})
	mux.HandleFunc("/admin/snapshot", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		if err := database.Snapshot(); err != nil {
			writeDatabaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "snapshot created"})
	})
	mux.HandleFunc("/admin/storage", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, database.StorageMetrics())
	})
	return mux
}

func parseUintPath(path, prefix string) (uint64, error) {
	value := strings.TrimPrefix(path, prefix)
	if value == "" || strings.Contains(value, "/") {
		return 0, errors.New("invalid key")
	}
	key, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, errors.New("invalid key")
	}
	return key, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	return false
}

func writeDatabaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrShardUnavailable):
		writeError(w, http.StatusServiceUnavailable, err)
	case errors.Is(err, db.ErrFailureActive):
		writeError(w, http.StatusConflict, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
