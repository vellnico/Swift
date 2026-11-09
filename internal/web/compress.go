package web

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
)

// gzipCache compresses embedded files once; they never change while serving.
type gzipCache struct{ m sync.Map }

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		enc, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(enc), "gzip") {
			return strings.ReplaceAll(params, " ", "") != "q=0"
		}
	}
	return false
}

func compressible(name string) bool {
	switch path.Ext(name) {
	case ".js", ".css", ".html", ".svg", ".json", ".txt":
		return true
	}
	return false
}

func gzipBytes(b []byte) []byte {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = zw.Write(b)
	_ = zw.Close()
	return buf.Bytes()
}

// serveFile sends an embedded file, gzipped when the client accepts it.
func (c *gzipCache) serveFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	if !compressible(name) {
		http.ServeFileFS(w, r, fsys, name)
		return
	}
	w.Header().Add("Vary", "Accept-Encoding")
	if !acceptsGzip(r) {
		http.ServeFileFS(w, r, fsys, name)
		return
	}
	var gz []byte
	if v, ok := c.m.Load(name); ok {
		gz = v.([]byte)
	} else {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			http.ServeFileFS(w, r, fsys, name)
			return
		}
		gz = gzipBytes(raw)
		c.m.Store(name, gz)
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" && w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", ct)
	}
	writeGzipped(w, r, gz)
}

func writeGzipped(w http.ResponseWriter, r *http.Request, gz []byte) {
	h := w.Header()
	h.Set("Content-Encoding", "gzip")
	h.Set("Content-Length", strconv.Itoa(len(gz)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(gz)
	}
}

// writeBody sends a generated response, gzipped when worthwhile.
func writeBody(w http.ResponseWriter, r *http.Request, body []byte) {
	w.Header().Add("Vary", "Accept-Encoding")
	if len(body) < 1024 || !acceptsGzip(r) {
		_, _ = w.Write(body)
		return
	}
	writeGzipped(w, r, gzipBytes(body))
}
