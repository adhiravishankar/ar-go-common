package common

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DatabaseConfig holds optimized MongoDB connection settings
type DatabaseConfig struct {
	MaxPoolSize            uint64
	MinPoolSize            uint64
	MaxConnIdleTime        time.Duration
	MaxConnecting          uint64
	HeartbeatInterval      time.Duration
	ServerSelectionTimeout time.Duration
	SocketTimeout          time.Duration
	ConnectTimeout         time.Duration
}

// DefaultDatabaseConfig returns optimized database configuration
func DefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		MaxPoolSize:            25,               // Reduced from 50-100
		MinPoolSize:            5,                // Reduced from 10
		MaxConnIdleTime:        5 * time.Minute,  // Reduced idle time
		MaxConnecting:          5,                // Limit concurrent connections
		HeartbeatInterval:      60 * time.Second, // Increased heartbeat
		ServerSelectionTimeout: 3 * time.Second,  // Faster timeout
		SocketTimeout:          15 * time.Second, // Shorter socket timeout
		ConnectTimeout:         5 * time.Second,  // Shorter connect timeout
	}
}

// NewOptimizedClient creates a MongoDB client with memory-optimized settings
// If uri is empty, it will use the MONGODB_URL environment variable
// If config is nil, it will use the default configuration
func NewOptimizedClient(uri string, config *DatabaseConfig) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use environment variable if URI is not provided
	if uri == "" {
		uri = os.Getenv("MONGODB_URL")
		if uri == "" {
			return nil, fmt.Errorf("MongoDB URI not provided and MONGODB_URL environment variable is not set")
		}
	}

	// Use default configuration if not provided
	var cfg DatabaseConfig
	if config != nil {
		cfg = *config
	} else {
		cfg = *DefaultDatabaseConfig()
	}

	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetMinPoolSize(cfg.MinPoolSize).
		SetMaxConnIdleTime(cfg.MaxConnIdleTime).
		SetMaxConnecting(cfg.MaxConnecting).
		SetHeartbeatInterval(cfg.HeartbeatInterval).
		SetServerSelectionTimeout(cfg.ServerSelectionTimeout).
		SetSocketTimeout(cfg.SocketTimeout).
		SetConnectTimeout(cfg.ConnectTimeout).
		SetRetryWrites(true).
		SetRetryReads(true)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("MongoDB connection error: %w", err)
	}

	// Verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("MongoDB ping failed: %w", err)
	}

	log.Println("MongoDB client connected with optimized settings")
	return client, nil
}

// GetPictureCountsForEntities returns a map of entityID to picture count using optimized aggregation
func GetPictureCountsForEntities(ctx context.Context, entityIDs []string, entityField string, collection *mongo.Collection) map[string]uint64 {
	if len(entityIDs) == 0 {
		return make(map[string]uint64)
	}

	// Use more efficient aggregation pipeline
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{entityField: bson.M{"$in": entityIDs}}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$" + entityField,
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$project", Value: bson.M{
			"_id":   1,
			"count": 1,
		}}},
	}

	opts := options.Aggregate().
		SetBatchSize(100).
		SetMaxTime(30 * time.Second) // Prevent long-running queries

	cursor, err := collection.Aggregate(ctx, pipeline, opts)
	if err != nil {
		log.Printf("Aggregation error: %v", err)
		return make(map[string]uint64)
	}

	safeCursor := NewSafeCursor(cursor, ctx)
	defer safeCursor.Close()

	counts := make(map[string]uint64, len(entityIDs))

	for safeCursor.Next() {
		var result struct {
			ID    string `bson:"_id"`
			Count uint64 `bson:"count"`
		}
		if err := safeCursor.Decode(&result); err != nil {
			log.Printf("Decode error: %v", err)
			continue
		}
		counts[result.ID] = result.Count
	}

	if err := safeCursor.Err(); err != nil {
		log.Printf("Cursor iteration error: %v", err)
	}

	return counts
}

// OptimizedFindWithOptions performs a find operation with custom options and safe cursor handling
func FindWithOptions(ctx context.Context, collection *mongo.Collection, filter bson.M, opts *options.FindOptions, capacity int) (*SafeCursor, error) {
	// Set default batch size if not specified
	if opts.BatchSize == nil {
		batchSize := int32(100)
		opts.SetBatchSize(batchSize)
	}

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("find operation failed: %w", err)
	}

	return NewSafeCursor(cursor, ctx), nil
}
