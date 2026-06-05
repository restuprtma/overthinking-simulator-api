package client

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseRecord parses one IQPlus frame line (without trailing CR/LF).
//
// Format: IQP|Date|Time|Sequence#|RecordType|Data
func ParseRecord(line string) (Record, error) {
	if !strings.HasPrefix(line, "IQP|") {
		return Record{}, fmt.Errorf("missing IQP header")
	}
	parts := strings.SplitN(line, "|", 6)
	if len(parts) < 6 {
		return Record{}, fmt.Errorf("insufficient fields: got %d", len(parts))
	}
	seq, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return Record{}, fmt.Errorf("invalid sequence %q: %w", parts[3], err)
	}
	rt, err := strconv.Atoi(parts[4])
	if err != nil {
		return Record{}, fmt.Errorf("invalid record type %q: %w", parts[4], err)
	}
	return Record{
		Date:       parts[1],
		Time:       parts[2],
		Sequence:   seq,
		RecordType: rt,
		RawData:    parts[5],
		ReceivedAt: time.Now(),
	}, nil
}
