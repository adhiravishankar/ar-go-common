package common

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/mongo"
)

// SafeCursor wraps mongo.Cursor with automatic cleanup and better error handling
type SafeCursor struct {
	cursor *mongo.Cursor
	ctx    context.Context
}

// NewSafeCursor creates a new safe cursor wrapper
func NewSafeCursor(cursor *mongo.Cursor, ctx context.Context) *SafeCursor {
	return &SafeCursor{cursor: cursor, ctx: ctx}
}

// Close closes the cursor and logs any errors
func (sc *SafeCursor) Close() {
	if sc.cursor != nil {
		if err := sc.cursor.Close(sc.ctx); err != nil {
			log.Printf("Error closing cursor: %v", err)
		}
	}
}

// Next advances the cursor to the next document
func (sc *SafeCursor) Next() bool {
	return sc.cursor.Next(sc.ctx)
}

// Decode decodes the current document into the provided value
func (sc *SafeCursor) Decode(val interface{}) error {
	return sc.cursor.Decode(val)
}

// Err returns any error that occurred during cursor iteration
func (sc *SafeCursor) Err() error {
	return sc.cursor.Err()
}

// All decodes all documents into the provided slice
func (sc *SafeCursor) All(results interface{}) error {
	return sc.cursor.All(sc.ctx, results)
}
