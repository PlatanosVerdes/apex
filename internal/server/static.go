package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

// The PNG the Dockerfile renders from web/icon.svg, since iOS ignores SVG icons on the home screen.
const touchIconPath = "/usr/local/share/apex/apple-touch-icon.png"

type asset struct {
	body, gzipped []byte
	etag, ctype   string
}

// assets reads every embedded file once, with its gzip and an ETag from its content, so a visit
// downloads about a third of the bytes and a return visit only what changed.
func assets(static fs.FS) map[string]*asset {
	out := map[string]*asset{}
	_ = fs.WalkDir(static, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(static, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		a := &asset{body: body, etag: `"` + hex.EncodeToString(sum[:8]) + `"`, ctype: mime.TypeByExtension(path.Ext(p))}
		if a.ctype == "" {
			a.ctype = "application/octet-stream"
		}
		if compressible(a.ctype) {
			var b bytes.Buffer
			z, _ := gzip.NewWriterLevel(&b, gzip.BestCompression)
			_, _ = z.Write(body)
			_ = z.Close()
			if b.Len() < len(body) {
				a.gzipped = b.Bytes()
			}
		}
		out[p] = a
		return nil
	})
	return out
}

func compressible(ctype string) bool {
	return strings.HasPrefix(ctype, "text/") || strings.Contains(ctype, "javascript") ||
		strings.Contains(ctype, "json") || strings.Contains(ctype, "svg")
}

// serveAsset answers one file. The libraries under vendor/ carry their version in the name, so a
// browser may keep them for a year; the rest are checked against their ETag on every visit.
func serveAsset(w http.ResponseWriter, r *http.Request, name string, a *asset) {
	h := w.Header()
	h.Set("Content-Type", a.ctype)
	h.Set("ETag", a.etag)
	h.Set("Vary", "Accept-Encoding")
	if strings.HasPrefix(name, "vendor/") {
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "no-cache")
	}
	if r.Header.Get("If-None-Match") == a.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body := a.body
	if a.gzipped != nil && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		h.Set("Content-Encoding", "gzip")
		body = a.gzipped
	}
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

// gzipJSON compresses the API's answers for clients that take gzip; a long route's line is the
// bulk of what the page asks for.
func gzipJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Vary", "Accept-Encoding")
		gz := &gzipWriter{ResponseWriter: w}
		next.ServeHTTP(gz, r)
		gz.close()
	})
}

type gzipWriter struct {
	http.ResponseWriter
	z       *gzip.Writer
	started bool
	plain   bool
}

func (g *gzipWriter) WriteHeader(code int) {
	if !g.started {
		g.started = true
		// Errors and empty answers go as they are; only JSON bodies are worth compressing.
		if code != http.StatusOK || !strings.Contains(g.Header().Get("Content-Type"), "json") {
			g.plain = true
		} else {
			g.Header().Del("Content-Length")
			g.Header().Set("Content-Encoding", "gzip")
			g.z = gzip.NewWriter(g.ResponseWriter)
		}
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	if !g.started {
		g.WriteHeader(http.StatusOK)
	}
	if g.plain {
		return g.ResponseWriter.Write(b)
	}
	return g.z.Write(b)
}

func (g *gzipWriter) close() {
	if g.z != nil {
		_ = g.z.Close()
	}
}
