// Copyright (c) 2021 Changkun Ou <hi@changkun.de>. All Rights Reserved.
// Unauthorized using, copying, modifying and distributing, via any
// medium is strictly prohibited.

package void

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"changkun.de/x/void/internal/uuid"
	"go.etcd.io/bbolt"
	"golang.design/x/tgstore"
	"golang.org/x/crypto/chacha20poly1305"
)

const bucket = "files"

type Response struct {
	Id      string `json:"id"`
	Message string `json:"message"`
}

type Metadata struct {
	Id       string `json:"id"`
	UploadId string `json:"upload_id"`
	FileName string `json:"filename"`
	FileSize int64  `json:"filesize"`
	Key      []byte `json:"key"`
}

func (m *Metadata) String() string {
	return fmt.Sprintf("%s\t%s\t%d\t%s", m.Id, m.FileName, m.FileSize, m.UploadId)
}

type Server struct {
	store *tgstore.TGStore
	db    *bbolt.DB
}

func NewServer() *Server {
	db, err := bbolt.Open(Conf.DB, 0666, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatalf("cannot open void.db: %v", err)
	}

	s := &Server{store: tgstore.New(), db: db}
	s.store.BotToken = Conf.BotToken
	s.store.ChatID = Conf.ChatID
	return s
}

func (s *Server) Run() {
	l := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer log.Println(readIP(r), r.Method, r.URL.Path, r.URL.RawQuery)
			next.ServeHTTP(w, r)
		})
	}

	http.Handle("/void", l(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer func() {
			if err == nil {
				return
			}

			if !errors.Is(err, errUnauthorized) {
				w.WriteHeader(http.StatusBadRequest)
			}
			w.Header().Set("Content-Type", "application/json")
			b, _ := json.Marshal(Response{Message: err.Error()})
			w.Write(b)
			log.Println(err)
		}()

		switch r.Method {
		case http.MethodDelete:
			s.handleDelete(w, r)
		case http.MethodGet:
			s.handleGet(w, r)
		case http.MethodPost:
			// All request must be authenticated.
			user, _, err := s.handleAuth(w, r)
			if err != nil {
				return
			}
			log.Println("login:", user)
			s.handlePost(w, r)
		default:
			err := fmt.Errorf("%s is not supported", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
		}
	})))

	ss := &http.Server{Addr: Conf.Port, Handler: nil}
	go func() {
		log.Printf("void server is running at %v/void\n", Conf.Port)
		if err := ss.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ss.Shutdown(ctx); err != nil {
		log.Fatal("forced to shutdown: ", err)
	}

	log.Println("server exiting, good bye!")
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) (err error) {
	id := r.URL.Query().Get("id")
	if id == "" {
		err = errors.New("missing id for the delete")
		return
	}

	return s.db.Update(func(t *bbolt.Tx) error {
		b := t.Bucket([]byte(bucket))
		return b.Delete([]byte(id))
	})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) (err error) {
	id := r.URL.Query().Get("id")
	if id == "" {
		err = s.handleList(w, r)
		return
	}

	meta := &Metadata{}
	if err = s.db.View(func(t *bbolt.Tx) error {
		b := t.Bucket([]byte(bucket))
		v := b.Get([]byte(id))
		return json.Unmarshal(v, meta)
	}); err != nil {
		return
	}
	if meta.UploadId == "" {
		err = fmt.Errorf("id does not exist")
		return
	}

	// Data mode: return upload id and key.
	if r.URL.Query().Get("mode") == "data" {
		_, _, err = s.handleAuth(w, r)
		if err != nil {
			return
		}

		b, _ := json.Marshal(meta)
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(b)
		return
	}

	var f io.ReadSeekCloser
	f, err = s.store.Download(r.Context(), meta.Key, meta.UploadId)
	if err != nil {
		err = fmt.Errorf("download with error: %w", err)
		return
	}
	defer f.Close()

	w.Header().Add("Content-Disposition", `attachment; filename="`+meta.FileName+`"`)
	w.Header().Add("Content-Length", strconv.FormatInt(meta.FileSize, 10))
	_, err = io.Copy(w, f)
	if err != nil {
		err = fmt.Errorf("download with error: %w", err)
		return
	}
	return
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) (err error) {
	raw := false
	if r.URL.Query().Get("mode") == "data" {
		_, _, err = s.handleAuth(w, r)
		if err != nil {
			return
		}
		raw = true
	}

	var files []*Metadata
	if err = s.db.View(func(t *bbolt.Tx) error {
		b := t.Bucket([]byte(bucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			file := &Metadata{}
			err := json.Unmarshal(v, file)
			if err != nil {
				return err
			}
			files = append(files, file)
		}

		return nil
	}); err != nil {
		return
	}
	if raw {
		b, _ := json.Marshal(files)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
		return
	}

	err = voidTmpl.Execute(w, struct{ All []*Metadata }{files})
	if err != nil {
		err = fmt.Errorf("failed to render template: %w", err)
		return
	}
	return nil
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) (err error) {
	var f multipart.File
	var h *multipart.FileHeader

	f, h, err = r.FormFile("file")
	if err != nil {
		err = fmt.Errorf("uploaded file contains error: %w", err)
		return
	}
	key := make([]byte, chacha20poly1305.KeySize)
	_, err = rand.Read(key)
	if err != nil {
		err = fmt.Errorf("generate key error: %w", err)
		return
	}

	m := &Metadata{
		FileName: h.Filename,
		FileSize: h.Size,
		Key:      key,
	}

	m.UploadId, err = s.store.Upload(r.Context(), m.Key, f)
	if err != nil {
		err = fmt.Errorf("upload failed with error: %w", err)
		return
	}

	m.Id = uuid.Must(uuid.NewShort())
	if err = s.db.Update(func(t *bbolt.Tx) error {
		b := t.Bucket([]byte(bucket))
		d, _ := json.Marshal(m)
		return b.Put([]byte(m.Id), d)
	}); err != nil {
		return
	}

	b, _ := json.Marshal(Response{
		Id:      m.Id,
		Message: fmt.Sprintf("Upload file %s success.", h.Filename),
	})
	_, err = w.Write(b)
	return
}

// readIP implements a best effort approach to return the real client IP.
func readIP(r *http.Request) (ip string) {
	ip = r.Header.Get("X-Forwarded-For")
	ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	if ip == "" {
		ip = strings.TrimSpace(r.Header.Get("X-Real-Ip"))
	}
	if ip != "" {
		return ip
	}
	ip = r.Header.Get("X-Appengine-Remote-Addr")
	if ip != "" {
		return ip
	}
	ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return "unknown" // use unknown to guarantee non empty string
	}
	return ip
}

var voidTmpl = template.Must(template.New("files").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>changkun.de's void file sharing system</title>
<style>
html, body {
	font-family: sans-serif, monospace;
	background-color: #333;
	overflow: hidden;
}
body {
	color: #aaa;
	margin: 30px 40px 30px;
}
a {
	text-decoration: none;
	color: #aaa;
}
a:visited {
	color: #aaa;
}
a:hover {
	color: #3c9ae8;
}
table {
	width: 100%;
}
tr {
	line-height: 30px;
}
th {
	text-align: left;
}
footer {
	margin-top: 30px;
	bottom: 2%;
}
</style>
</head>
<body>
<h1>The Void File Sharing System</h1>
<p>void is a zero storage cost file sharing system.</p>

<table class="table">
<tr><th>ID</th><th>File Name</th><th>File Size</th></tr>
{{range .All}}
<tr><td>{{.Id}}</td><td><a href="/void?id={{.Id}}">{{.FileName}}</a></td><td>{{.FileSize}}</td></tr>
{{end}}
</table>

<footer>
<a href="/s/void">void</a> &copy; 2021 Created by Changkun Ou.
</footer>
</body>
</html>
`))
