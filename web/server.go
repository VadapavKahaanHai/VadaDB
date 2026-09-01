package web

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"

	"vadadb/db"
	"vadadb/structures"
)

//go:embed templates/index.html
var templates embed.FS

type Server struct {
	database *db.Database
	template *template.Template
}

type pageView struct {
	Shards    []db.ShardInfo
	Records   []db.Record
	ScanError string
	Fusion    [][]structures.FusedKVNode
	Storage   db.StorageMetrics
	Notice    string
}

func New(database *db.Database) (*Server, error) {
	parsed, err := template.ParseFS(templates, "templates/index.html")
	if err != nil {
		return nil, err
	}
	return &Server{database: database, template: parsed}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("POST /put", s.put)
	mux.HandleFunc("POST /get", s.get)
	mux.HandleFunc("POST /delete", s.delete)
	mux.HandleFunc("POST /crash", s.crash)
	mux.HandleFunc("POST /recover", s.recover)
	mux.HandleFunc("POST /snapshot", s.snapshot)
	return mux
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	records, err := s.database.Scan()
	view := pageView{Shards: s.database.Shards(), Records: records, Fusion: s.database.FusionSnapshot(), Storage: s.database.StorageMetrics(), Notice: r.URL.Query().Get("notice")}
	if err != nil {
		view.ScanError = err.Error()
	}
	if err := s.template.ExecuteTemplate(w, "index.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) put(w http.ResponseWriter, r *http.Request) {
	key, err := parseUintForm(w, r, "key")
	if err != nil {
		badRequest(w, err)
		return
	}
	value, err := strconv.ParseUint(r.FormValue("value"), 10, 64)
	if err != nil {
		badRequest(w, errors.New("value must be an unsigned decimal integer"))
		return
	}
	if err := s.database.Put(key, value); err != nil {
		writeDatabaseError(w, err)
		return
	}
	redirect(w, r, fmt.Sprintf("PUT %d = %d committed to shard %d and fusion", key, value, s.database.Route(key)))
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	key, err := parseUintForm(w, r, "key")
	if err != nil {
		badRequest(w, err)
		return
	}
	value, found, err := s.database.Get(key)
	if err != nil {
		writeDatabaseError(w, err)
		return
	}
	if !found {
		redirect(w, r, fmt.Sprintf("GET %d: key not found", key))
		return
	}
	redirect(w, r, fmt.Sprintf("GET %d = %d from shard %d", key, value, s.database.Route(key)))
}

func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	key, err := parseUintForm(w, r, "key")
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := s.database.Delete(key); err != nil {
		writeDatabaseError(w, err)
		return
	}
	redirect(w, r, fmt.Sprintf("DELETE %d committed", key))
}

func (s *Server) crash(w http.ResponseWriter, r *http.Request) {
	id, err := parseShardForm(w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := s.database.CrashShard(id); err != nil {
		writeDatabaseError(w, err)
		return
	}
	redirect(w, r, fmt.Sprintf("Shard %d crashed; its in-memory records were discarded", id))
}

func (s *Server) recover(w http.ResponseWriter, r *http.Request) {
	id, err := parseShardForm(w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := s.database.RecoverShard(id); err != nil {
		writeDatabaseError(w, err)
		return
	}
	redirect(w, r, fmt.Sprintf("Shard %d recovered from fusion plus healthy shards", id))
}

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	if err := parseForm(w, r); err != nil {
		badRequest(w, err)
		return
	}
	if err := s.database.Snapshot(); err != nil {
		writeDatabaseError(w, err)
		return
	}
	redirect(w, r, "Snapshot created")
}

func parseUintForm(w http.ResponseWriter, r *http.Request, name string) (uint64, error) {
	if err := parseForm(w, r); err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(r.FormValue(name), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned decimal integer", name)
	}
	return value, nil
}

func parseShardForm(w http.ResponseWriter, r *http.Request) (int, error) {
	if err := parseForm(w, r); err != nil {
		return 0, err
	}
	id, err := strconv.Atoi(r.FormValue("shard"))
	if err != nil || id < 0 {
		return 0, errors.New("shard must be a non-negative integer")
	}
	return id, nil
}

func parseForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	return r.ParseForm()
}

func redirect(w http.ResponseWriter, r *http.Request, notice string) {
	http.Redirect(w, r, "/?notice="+url.QueryEscape(notice), http.StatusSeeOther)
}

func badRequest(w http.ResponseWriter, err error) { http.Error(w, err.Error(), http.StatusBadRequest) }

func writeDatabaseError(w http.ResponseWriter, err error) {
	status := http.StatusConflict
	if !errors.Is(err, db.ErrShardUnavailable) && !errors.Is(err, db.ErrFailureActive) {
		status = http.StatusBadRequest
	}
	http.Error(w, err.Error(), status)
}
