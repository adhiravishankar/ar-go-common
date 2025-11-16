package common

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgraph-io/ristretto"
)

// Cache TTL constants for different data types
const (
	// Short TTL for frequently changing data
	CacheTTLShort = 6 * time.Hour
	// Medium TTL for moderately stable data
	CacheTTLMedium = 12 * time.Hour
	// Long TTL for relatively stable data
	CacheTTLLong = 24 * time.Hour
	// HTTP responses cache (longer for admin operations)
	CacheTTLHTTP = 2 * time.Hour
)

// SetCacheWithTTLBinary stores a value using gob encoding (more efficient than JSON)
func SetCacheWithTTLBinary(cache *ristretto.Cache, key string, value interface{}, ttl time.Duration) bool {
	if cache == nil {
		log.Printf("Cache not initialized, skipping set for key: %s", key)
		return false
	}

	// Use gob encoding for better performance than JSON
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(value); err != nil {
		log.Printf("Failed to encode value for cache key %s: %v", key, err)
		return false
	}

	data := buf.Bytes()
	cost := int64(len(data))

	success := cache.SetWithTTL(key, data, cost, ttl)
	if success {
		log.Printf("Cached item with key: %s (cost: %d, ttl: %v)", key, cost, ttl)
	} else {
		log.Printf("Failed to cache item with key: %s", key)
	}

	return success
}

// GetCacheBinary retrieves and decodes a gob-encoded value
func GetCacheBinary(cache *ristretto.Cache, key string, target interface{}) bool {
	if cache == nil {
		return false
	}

	value, found := cache.Get(key)
	if !found {
		return false
	}

	data, ok := value.([]byte)
	if !ok {
		log.Printf("Invalid cache value type for key: %s", key)
		return false
	}

	buf := bytes.NewReader(data)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(target); err != nil {
		log.Printf("Failed to decode cached value for key %s: %v", key, err)
		return false
	}

	log.Printf("Cache hit for key: %s", key)
	return true
}

// Global pool instance
var writerPool = NewCacheResponseWriterPool(50)

// CacheMiddleware provides HTTP middleware for caching GET requests with memory optimizations
// If customWriterPool is nil, it will use the global writerPool instance
func CacheMiddleware(cache *ristretto.Cache, ttl time.Duration, customWriterPool *CacheResponseWriterPool) func(http.Handler) http.Handler {
	// Use global pool if none provided
	poolToUse := customWriterPool
	if poolToUse == nil {
		poolToUse = writerPool
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only cache GET requests
			if r.Method != "GET" {
				next.ServeHTTP(w, r)
				return
			}

			// Skip caching for certain paths or if cache is disabled
			if cache == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Generate cache key from request path and query parameters
			cacheKey := CacheKey("http", r.URL.Path, r.URL.RawQuery)

			// Try to get from cache
			var cachedResponse CachedResponse
			if GetCache(cache, cacheKey, &cachedResponse) {
				// Set headers
				for key, value := range cachedResponse.Headers {
					w.Header().Set(key, value)
				}
				w.Header().Set("X-Cache", "HIT")

				// Return cached response
				w.WriteHeader(cachedResponse.StatusCode)
				w.Write(cachedResponse.Body)
				return
			}

			// Get writer from pool
			writer := poolToUse.Get(w)
			defer poolToUse.Put(writer) // Return to pool when done

			// Process request
			next.ServeHTTP(writer, r)

			// Cache the response if it was successful
			if writer.statusCode >= 200 && writer.statusCode < 300 {
				cachedResponse := CachedResponse{
					StatusCode:  writer.statusCode,
					ContentType: writer.Header().Get("Content-Type"),
					Headers:     writer.headers,
					Body:        make([]byte, len(writer.body)), // Copy to avoid reference issues
				}
				copy(cachedResponse.Body, writer.body)

				SetCacheWithTTL(cache, cacheKey, cachedResponse, ttl)
			}

			// Add cache miss header
			writer.Header().Set("X-Cache", "MISS")
		})
	}
}

// GetCache retrieves a value from the cache and unmarshals it into the target
func GetCache(cache *ristretto.Cache, key string, target interface{}) bool {
	if cache == nil {
		return false
	}

	value, found := cache.Get(key)
	if !found {
		return false
	}

	jsonValue, ok := value.([]byte)
	if !ok {
		log.Printf("Invalid cache value type for key: %s", key)
		return false
	}

	err := json.Unmarshal(jsonValue, target)
	if err != nil {
		log.Printf("Failed to unmarshal cached value for key %s: %v", key, err)
		return false
	}

	log.Printf("Cache hit for key: %s", key)
	return true
}

// SetCache stores a value in the cache with default TTL
func SetCache(cache *ristretto.Cache, key string, value interface{}) bool {
	return SetCacheWithTTL(cache, key, value, 24*time.Hour)
}

// SetCacheWithTTL stores a value in the cache with specified TTL
func SetCacheWithTTL(cache *ristretto.Cache, key string, value interface{}, ttl time.Duration) bool {
	if cache == nil {
		log.Printf("Cache not initialized, skipping set for key: %s", key)
		return false
	}

	// Serialize the value to JSON for consistent storage
	jsonValue, err := json.Marshal(value)
	if err != nil {
		log.Printf("Failed to marshal value for cache key %s: %v", key, err)
		return false
	}

	// Calculate cost based on JSON size (rough estimate)
	cost := int64(len(jsonValue))

	success := cache.SetWithTTL(key, jsonValue, cost, ttl)
	if success {
		log.Printf("Cached item with key: %s (cost: %d, ttl: %v)", key, cost, ttl)
	} else {
		log.Printf("Failed to cache item with key: %s", key)
	}

	return success
}

// Alternative implementation using join for even better performance
func CacheKeyJoin(components ...string) string {
	return strings.Join(components, ":")
}

// CacheKey generates a consistent cache key from components using optimized string concatenation
// This reduces memory allocations from O(n²) to O(n)
func CacheKey(components ...string) string {
	if len(components) == 0 {
		return ""
	}

	if len(components) == 1 {
		return components[0]
	}

	// Use strings.Join for optimal performance
	return strings.Join(components, ":")
}

// Delete removes a value from the cache
func CacheDelete(cache *ristretto.Cache, key string) {
	if cache == nil {
		return
	}

	cache.Del(key)
	log.Printf("Deleted cache key: %s", key)
}

// Clear clears all items from the cache
func CacheClear(cache *ristretto.Cache) {
	if cache == nil {
		return
	}

	cache.Clear()
	log.Println("Cache cleared")
}

// GetMetrics returns cache metrics if available
func CacheGetMetrics(cache *ristretto.Cache) *ristretto.Metrics {
	if cache == nil {
		return nil
	}

	return cache.Metrics
}
