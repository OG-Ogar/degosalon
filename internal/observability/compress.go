package observability

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

const minCompressibleBytes = 512

var gzipWriterPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// isCompressibleType reports whether the MIME type is suitable for gzip compression.
func isCompressibleType(contentType string) bool {
	if contentType == "" {
		return false
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	switch mediaType {
	case "application/javascript",
		"application/x-javascript",
		"application/json",
		"application/xml",
		"application/xhtml+xml",
		"image/svg+xml",
		"application/wasm":
		return true
	default:
		return false
	}
}

type compressResponseWriter struct {
	http.ResponseWriter
	gzWriter    *gzip.Writer
	buf         bytes.Buffer
	statusCode  int
	wroteHeader bool
	compressing bool
	disabled    bool
}

func (w *compressResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *compressResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.statusCode = status
}

func (w *compressResponseWriter) decide(sample []byte) {
	if w.wroteHeader {
		return
	}

	ct := w.Header().Get("Content-Type")
	if ct == "" && len(sample) > 0 {
		ct = http.DetectContentType(sample)
		w.Header().Set("Content-Type", ct)
	}

	// Never compress if the handler already encoded the body or content is not compressible.
	if w.disabled || w.Header().Get("Content-Encoding") != "" || !isCompressibleType(ct) {
		w.compressing = false
		w.commitHeader()
		return
	}

	w.compressing = true
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Encoding", "gzip")

	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(w.ResponseWriter)
	w.gzWriter = gz

	w.commitHeader()
}

func (w *compressResponseWriter) commitHeader() {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	w.ResponseWriter.WriteHeader(w.statusCode)
}

func (w *compressResponseWriter) Write(p []byte) (int, error) {
	if w.compressing {
		return w.gzWriter.Write(p)
	}

	if w.disabled || w.wroteHeader {
		return w.ResponseWriter.Write(p)
	}

	// Buffer initial bytes to avoid compressing tiny payloads.
	w.buf.Write(p)
	if w.buf.Len() >= minCompressibleBytes {
		w.decide(w.buf.Bytes())
		if w.compressing {
			_, err := w.gzWriter.Write(w.buf.Bytes())
			w.buf.Reset()
			return len(p), err
		}
		// Not compressing: flush buffered bytes uncompressed
		w.commitHeader()
		n, err := w.ResponseWriter.Write(w.buf.Bytes())
		w.buf.Reset()
		if err != nil {
			return n, err
		}
		return len(p), nil
	}

	return len(p), nil
}

func (w *compressResponseWriter) Flush() {
	if !w.wroteHeader {
		w.decide(w.buf.Bytes())
		if !w.compressing {
			w.commitHeader()
			if w.buf.Len() > 0 {
				_, _ = w.ResponseWriter.Write(w.buf.Bytes())
				w.buf.Reset()
			}
		} else {
			if w.buf.Len() > 0 {
				_, _ = w.gzWriter.Write(w.buf.Bytes())
				w.buf.Reset()
			}
		}
	} else if w.compressing && w.buf.Len() > 0 {
		_, _ = w.gzWriter.Write(w.buf.Bytes())
		w.buf.Reset()
	}

	if w.compressing && w.gzWriter != nil {
		_ = w.gzWriter.Flush()
	}

	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *compressResponseWriter) close() {
	if !w.wroteHeader {
		// Response is smaller than minCompressibleBytes; check if compressible
		ct := w.Header().Get("Content-Type")
		if ct == "" && w.buf.Len() > 0 {
			ct = http.DetectContentType(w.buf.Bytes())
			w.Header().Set("Content-Type", ct)
		}

		// Only compress if compressible and non-empty
		if !w.disabled && w.Header().Get("Content-Encoding") == "" && isCompressibleType(ct) && w.buf.Len() > 0 {
			w.compressing = true
			w.Header().Del("Content-Length")
			w.Header().Set("Content-Encoding", "gzip")

			gz := gzipWriterPool.Get().(*gzip.Writer)
			gz.Reset(w.ResponseWriter)
			w.gzWriter = gz
			w.commitHeader()

			_, _ = gz.Write(w.buf.Bytes())
		} else {
			w.commitHeader()
			if w.buf.Len() > 0 {
				_, _ = w.ResponseWriter.Write(w.buf.Bytes())
			}
		}
	}

	if w.compressing && w.gzWriter != nil {
		_ = w.gzWriter.Close()
		gzipWriterPool.Put(w.gzWriter)
		w.gzWriter = nil
	}
}

// Compress returns an http.Handler that compresses responses with gzip when supported
// by the client and appropriate for the content type.
func Compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")

		if r.Method == http.MethodHead || strings.ToLower(r.Header.Get("Upgrade")) == "websocket" || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		cw := &compressResponseWriter{ResponseWriter: w}
		defer cw.close()

		next.ServeHTTP(cw, r)
	})
}
