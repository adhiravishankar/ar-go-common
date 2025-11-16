package common

import (
	"net/http"
)

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode  int               `json:"status_code"`
	ContentType string            `json:"content_type"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body"`
}

// CacheResponseWriter wraps http.ResponseWriter to capture response data
type CacheResponseWriter struct {
	http.ResponseWriter
	body       []byte
	statusCode int
	headers    map[string]string
}

func (w *CacheResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return w.ResponseWriter.Write(data)
}

func (w *CacheResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *CacheResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// CacheResponseWriterPool provides pooled response writers to reduce allocations
type CacheResponseWriterPool struct {
	pool chan *CacheResponseWriter
}

// NewCacheResponseWriterPool creates a new response writer pool
func NewCacheResponseWriterPool(size int) *CacheResponseWriterPool {
	return &CacheResponseWriterPool{
		pool: make(chan *CacheResponseWriter, size),
	}
}

// Get retrieves a response writer from the pool
func (p *CacheResponseWriterPool) Get(rw http.ResponseWriter) *CacheResponseWriter {
	select {
	case writer := <-p.pool:
		writer.ResponseWriter = rw
		writer.body = writer.body[:0] // Reset slice but keep capacity
		writer.statusCode = 200
		// Clear map but keep allocated space
		for k := range writer.headers {
			delete(writer.headers, k)
		}
		return writer
	default:
		return &CacheResponseWriter{
			ResponseWriter: rw,
			body:           make([]byte, 0, 1024), // Pre-allocate 1KB
			statusCode:     200,
			headers:        make(map[string]string, 8), // Pre-allocate for common headers
		}
	}
}

// Put returns a response writer to the pool
func (p *CacheResponseWriterPool) Put(writer *CacheResponseWriter) {
	// Only pool if body isn't too large (prevent memory bloat)
	if cap(writer.body) <= 8192 { // 8KB limit
		select {
		case p.pool <- writer:
		default:
			// Pool full, let it be garbage collected
		}
	}
}
