// Package sink writes assembled News documents to MongoDB.
//
// Document layout (collection `news`):
//
//	{
//	  _id:               "<news_id>",          // vendor-assigned, idempotent
//	  date:              ISODate("..."),       // UTC trade timestamp
//	  date_str:          "20211223",           // raw IQPlus YYYYMMDD
//	  time_str:          "185306",             // raw IQPlus HHMMSS (WIB)
//	  category:          "BIS",
//	  company_id:        "TLKM",
//	  headline:          "...",
//	  story:             "<full reassembled text>",
//	  packets_received:  4,
//	  num_packets:       4,
//	  inserted_at:       ISODate("..."),
//	  schema_version:    "v1"
//	}
//
// Indexes created on first connect (idempotent):
//   - date desc                        (recent news first)
//   - company_id asc + date desc       (per-stock timeline)
//   - text(headline, story)            (full-text search)
package sink

import (
	"context"
	"fmt"
	"time"

	"tuai/internal/modules/stock/news_consumer/assembler"
	"tuai/pkg/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// Config holds MongoDB connection params.
type Config struct {
	URI            string        // mongodb://user:pass@host:port/?authSource=...
	Database       string        // default "tuai"
	Collection     string        // default "news"
	ConnectTimeout time.Duration // default 5s
	WriteTimeout   time.Duration // default 5s
	SchemaVersion  string        // default "v1"
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Database:       "tuai",
		Collection:     "news",
		ConnectTimeout: 5 * time.Second,
		WriteTimeout:   5 * time.Second,
		SchemaVersion:  "v1",
	}
}

// MongoSink writes assembled news to MongoDB.
type MongoSink struct {
	cli  *mongo.Client
	coll *mongo.Collection
	cfg  Config
}

// New connects + pings + ensures indexes.
func New(ctx context.Context, cfg Config) (*MongoSink, error) {
	if cfg.Database == "" {
		cfg.Database = "tuai"
	}
	if cfg.Collection == "" {
		cfg.Collection = "news"
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 5 * time.Second
	}
	if cfg.SchemaVersion == "" {
		cfg.SchemaVersion = "v1"
	}
	if cfg.URI == "" {
		return nil, fmt.Errorf("mongo: URI required")
	}

	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	cli, err := mongo.Connect(connectCtx, options.Client().
		ApplyURI(cfg.URI).
		SetServerSelectionTimeout(cfg.ConnectTimeout))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	if err := cli.Ping(connectCtx, nil); err != nil {
		_ = cli.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	coll := cli.Database(cfg.Database).Collection(cfg.Collection)
	s := &MongoSink{cli: cli, coll: coll, cfg: cfg}

	if err := s.ensureIndexes(ctx); err != nil {
		_ = cli.Disconnect(context.Background())
		return nil, fmt.Errorf("ensure indexes: %w", err)
	}

	logger.Log.Info("mongo sink ready",
		zap.String("database", cfg.Database),
		zap.String("collection", cfg.Collection))
	return s, nil
}

// ensureIndexes creates the necessary indexes if they do not exist.
// Mongo's CreateMany is idempotent — existing indexes are skipped.
func (s *MongoSink) ensureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.ConnectTimeout)
	defer cancel()
	_, err := s.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "date", Value: -1}},
		},
		{
			Keys: bson.D{
				{Key: "company_id", Value: 1},
				{Key: "date", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "headline", Value: "text"},
				{Key: "story", Value: "text"},
			},
		},
	})
	return err
}

// Insert upserts one assembled News document. Idempotent on _id (news_id),
// so vendor resends and JetStream redeliveries are safe.
func (s *MongoSink) Insert(ctx context.Context, n assembler.News) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.WriteTimeout)
	defer cancel()

	doc := bson.M{
		"_id":              n.NewsID,
		"date":             n.Timestamp,
		"date_str":         n.Date,
		"time_str":         n.Time,
		"category":         n.Category,
		"company_id":       n.CompanyID,
		"headline":         n.Headline,
		"story":            n.Story,
		"packets_received": n.PacketsRecv,
		"num_packets":      n.NumPackets,
		"inserted_at":      time.Now().UTC(),
		"schema_version":   s.cfg.SchemaVersion,
	}

	opts := options.Replace().SetUpsert(true)
	_, err := s.coll.ReplaceOne(ctx, bson.M{"_id": n.NewsID}, doc, opts)
	return err
}

// Shutdown disconnects from MongoDB.
func (s *MongoSink) Shutdown(ctx context.Context) error {
	if s.cli == nil {
		return nil
	}
	disconnectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.cli.Disconnect(disconnectCtx)
}
