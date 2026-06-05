// Package iqplus_envelope holds the JSON contract that the IQPlus
// publisher emits per JetStream message. Every consumer (OHLCV, quote
// state cache, news indexer, alert engine, ...) deserializes into
// Envelope first.
//
// Source of truth: docs/JetStream/streams.md and the publisher's
// recordEnvelope in internal/modules/stock/iqplus_publisher/publisher.
//
// Bumping the schema requires:
//  1. update publisher.Config.SchemaVersion
//  2. update this struct
//  3. document the change in streams.md so downstream consumers know
package iqplus_envelope

import (
	"encoding/json"
	"time"
)

// Envelope is the JSON body of a JetStream message published by
// cmd/iqplus-publisher.
type Envelope struct {
	Date          string    `json:"date"`           // YYYYMMDD (server clock)
	Time          string    `json:"time"`           // HHMMSS  (server clock, WIB)
	Sequence      int64     `json:"sequence"`       // unique per day, starts at 1
	RecordType    int       `json:"record_type"`    // IQPlus record type (14 = quote, 15 = trade, ...)
	Data          string    `json:"data"`           // raw IQPlus payload (pipe/semi delimited)
	ReceivedAt    time.Time `json:"received_at"`    // publisher receive time (UTC)
	SchemaVersion string    `json:"schema_version"` // e.g. "v1"
	PublisherName string    `json:"publisher"`
}

// Unmarshal decodes a NATS message body into Envelope.
func Unmarshal(b []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(b, &e); err != nil {
		return Envelope{}, err
	}
	return e, nil
}
