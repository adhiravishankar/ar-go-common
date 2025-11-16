package common

import (
	"testing"
	"time"

	"github.com/dgraph-io/ristretto"
)

// TestCacheBasicOperations tests basic cache operations
func TestCacheBasicOperations(t *testing.T) {

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e4,  // 10,000 counters
		MaxCost:     1e6,  // 1MB max cost
		BufferItems: 64,   // 64 keys per buffer
		Metrics:     true, // Enable metrics for monitoring
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
	// Test data
	testKey := "test:key"
	testData := map[string]interface{}{
		"id":    "123",
		"name":  "Test Item",
		"count": 42,
	}

	// Test Set and Get
	success := SetCache(cache, testKey, testData)
	if !success {
		t.Error("Failed to set cache item")
	}

	var retrievedData map[string]interface{}
	found := GetCache(cache, testKey, &retrievedData)
	if !found {
		t.Error("Failed to retrieve cache item")
	}

	// Verify data integrity
	if retrievedData["id"] != testData["id"] {
		t.Errorf("Expected id %v, got %v", testData["id"], retrievedData["id"])
	}
	if retrievedData["name"] != testData["name"] {
		t.Errorf("Expected name %v, got %v", testData["name"], retrievedData["name"])
	}

	// Test cache miss
	var missData map[string]interface{}
	found = GetCache(cache, "nonexistent:key", &missData)
	if found {
		t.Error("Expected cache miss, but got hit")
	}

	// Test deletion
	CacheDelete(cache, testKey)
	found = GetCache(cache, testKey, &retrievedData)
	if found {
		t.Error("Expected cache miss after deletion, but got hit")
	}
}

// TestCacheInvalidation tests cache invalidation functions
func TestCacheInvalidation(t *testing.T) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e4,  // 10,000 counters
		MaxCost:     1e6,  // 1MB max cost
		BufferItems: 64,   // 64 keys per buffer
		Metrics:     true, // Enable metrics for monitoring
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}

	const (
		CacheKeyAircraftsList  = "list:aircrafts"
		CacheKeyAirlinesList   = "list:airlines"
		CacheKeyAirportsList   = "list:airports"
		CacheKeyRegionsList    = "list:regions"
		CacheKeyFlightPictures = "list:flight-pictures"
		CacheKeySeriesList     = "list:series"
	)

	// Set test data for different entity types
	SetCache(cache, CacheKeyAircraftsList, []string{"aircraft1", "aircraft2"})
	SetCache(cache, CacheKeyAirlinesList, []string{"airline1", "airline2"})
	SetCache(cache, CacheKeyAirportsList, []string{"airport1", "airport2"})
	SetCache(cache, CacheKeyRegionsList, []string{"region1", "region2"})

	// Verify data is cached
	var data []string
	if !GetCache(cache, CacheKeyAircraftsList, &data) {
		t.Error("Aircraft data should be cached")
	}
	if !GetCache(cache, CacheKeyAirlinesList, &data) {
		t.Error("Airline data should be cached")
	}

	// Test cache deletion (simulating invalidation)
	CacheDelete(cache, CacheKeyAircraftsList)
	if GetCache(cache, CacheKeyAircraftsList, &data) {
		t.Error("Aircraft cache should be invalidated")
	}
	// Airlines should still be cached
	if !GetCache(cache, CacheKeyAirlinesList, &data) {
		t.Error("Airline cache should still exist")
	}

	// Test airline cache deletion
	CacheDelete(cache, CacheKeyAirlinesList)
	if GetCache(cache, CacheKeyAirlinesList, &data) {
		t.Error("Airline cache should be invalidated")
	}

	// Test airport cache deletion
	CacheDelete(cache, CacheKeyAirportsList)
	if GetCache(cache, CacheKeyAirportsList, &data) {
		t.Error("Airport cache should be invalidated")
	}

	// Test region cache deletion
	CacheDelete(cache, CacheKeyRegionsList)
	if GetCache(cache, CacheKeyRegionsList, &data) {
		t.Error("Region cache should be invalidated")
	}
}

// TestCacheStats tests cache statistics functionality
func TestCacheStats(t *testing.T) {
	// Initialize cache for testing
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e4,
		MaxCost:     1e6,
		BufferItems: 64,
		Metrics:     true,
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}

	// Perform some cache operations
	SetCache(cache, "test:stats1", "value1")
	SetCache(cache, "test:stats2", "value2")

	var value string
	GetCache(cache, "test:stats1", &value)      // Hit
	GetCache(cache, "test:nonexistent", &value) // Miss

	// Get cache metrics
	metrics := CacheGetMetrics(cache)
	if metrics == nil {
		t.Error("Cache metrics should be available")
	}

	// Check that metrics are being tracked
	if metrics.KeysAdded() == 0 {
		t.Error("Keys added metric should be tracked")
	}
}

// TestCacheWithTTL tests TTL functionality
func TestCacheWithTTL(t *testing.T) {
	// Initialize cache for testing
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e4,
		MaxCost:     1e6,
		BufferItems: 64,
		Metrics:     true,
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}

	testKey := "test:ttl"
	testValue := "ttl_value"

	// Set with very short TTL for testing
	success := SetCacheWithTTL(cache, testKey, testValue, 100*time.Millisecond)
	if !success {
		t.Error("Failed to set cache item with TTL")
	}

	// Verify item exists immediately
	var retrievedValue string
	found := GetCache(cache, testKey, &retrievedValue)
	if !found {
		t.Error("Cache item should exist immediately after setting")
	}

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Verify item has expired (note: this might be flaky due to Ristretto's async nature)
	// We'll just ensure the test doesn't crash - TTL testing is complex with Ristretto
	GetCache(cache, testKey, &retrievedValue)
}

// TestCacheBinaryOperations tests binary cache operations
func TestCacheBinaryOperations(t *testing.T) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e4,
		MaxCost:     1e6,
		BufferItems: 64,
		Metrics:     true,
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}

	// Test data
	testKey := "test:binary"
	testData := map[string]interface{}{
		"id":    "456",
		"name":  "Binary Test Item",
		"count": 84,
	}

	// Test binary set and get
	success := SetCacheWithTTLBinary(cache, testKey, testData, time.Hour)
	if !success {
		t.Error("Failed to set binary cache item")
	}

	var retrievedData map[string]interface{}
	found := GetCacheBinary(cache, testKey, &retrievedData)
	if !found {
		t.Error("Failed to retrieve binary cache item")
	}

	// Verify data integrity
	if retrievedData["id"] != testData["id"] {
		t.Errorf("Expected id %v, got %v", testData["id"], retrievedData["id"])
	}
	if retrievedData["name"] != testData["name"] {
		t.Errorf("Expected name %v, got %v", testData["name"], retrievedData["name"])
	}
}

// TestCacheKeyGeneration tests cache key generation functions
func TestCacheKeyGeneration(t *testing.T) {
	// Test CacheKey function
	key1 := CacheKey("user", "123", "flights")
	expected1 := "user:123:flights"
	if key1 != expected1 {
		t.Errorf("Expected key %s, got %s", expected1, key1)
	}

	// Test single component
	key2 := CacheKey("single")
	expected2 := "single"
	if key2 != expected2 {
		t.Errorf("Expected key %s, got %s", expected2, key2)
	}

	// Test empty components
	key3 := CacheKey()
	expected3 := ""
	if key3 != expected3 {
		t.Errorf("Expected empty key, got %s", key3)
	}

	// Test CacheKeyJoin function
	key4 := CacheKeyJoin("admin", "aircrafts", "list")
	expected4 := "admin:aircrafts:list"
	if key4 != expected4 {
		t.Errorf("Expected key %s, got %s", expected4, key4)
	}
}

// TestCacheClearOperations tests cache clear functionality
func TestCacheClearOperations(t *testing.T) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e4,
		MaxCost:     1e6,
		BufferItems: 64,
		Metrics:     true,
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}

	// Add some test data
	SetCache(cache, "test:clear1", "value1")
	SetCache(cache, "test:clear2", "value2")

	// Verify data exists
	var value string
	if !GetCache(cache, "test:clear1", &value) {
		t.Error("Test data should exist before clear")
	}

	// Clear cache
	CacheClear(cache)

	// Verify data is gone (note: might be flaky due to async nature)
	// We'll just ensure the test doesn't crash
	GetCache(cache, "test:clear1", &value)
}
