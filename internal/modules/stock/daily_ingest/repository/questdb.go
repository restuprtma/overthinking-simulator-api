// Package repository owns the IO for the daily-ingest module.
//
// QuestDB side: aggregates raw ticks from `trades` into per-stock daily
// OHLCV via SAMPLE BY 1d ALIGN TO CALENDAR TIME ZONE 'Asia/Jakarta'.
// Result rows are converted to domain.DailyBar.
//
// Postgres side: batch UPSERT into stock.prices_daily.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"tuai/internal/modules/stock/daily_ingest/domain"
)

// QuestDBConfig points at the HTTP /exec endpoint and identifies which
// table/column we're aggregating.
//
// TsColumn defaults to "timestamp" — verify against your live schema with
// `SHOW COLUMNS FROM trades;` if SAMPLE BY queries fail.
type QuestDBConfig struct {
	BaseURL    string        // e.g. http://localhost:9000
	Table      string        // default "trades"
	TsColumn   string        // designated timestamp column, default "timestamp"
	HTTPTimout time.Duration // per-request timeout, default 60s

	// Optional auth — bearer token takes precedence if both set.
	BearerToken       string
	BasicAuthUser     string
	BasicAuthPassword string
}

// DefaultQuestDBConfig returns Phase-1 defaults.
func DefaultQuestDBConfig() QuestDBConfig {
	return QuestDBConfig{
		Table:      "trades",
		TsColumn:   "timestamp",
		HTTPTimout: 60 * time.Second,
	}
}

// QuestDBRepository wraps an HTTP client against the QuestDB /exec endpoint.
type QuestDBRepository struct {
	cfg QuestDBConfig
	cli *http.Client
}

func NewQuestDB(cfg QuestDBConfig) *QuestDBRepository {
	if cfg.Table == "" {
		cfg.Table = "trades"
	}
	if cfg.TsColumn == "" {
		cfg.TsColumn = "timestamp"
	}
	if cfg.HTTPTimout <= 0 {
		cfg.HTTPTimout = 60 * time.Second
	}
	return &QuestDBRepository{
		cfg: cfg,
		cli: &http.Client{Timeout: cfg.HTTPTimout},
	}
}

// FetchDailyBars returns one DailyBar per (stock, market, day) for trading
// days that fall within [startWIB, endWIBExclusive). The function builds the
// WIB-aligned SAMPLE BY 1d query, parses the JSON response, and yields domain
// rows.
//
// Market is read from the QuestDB `market` column per-row. Rows with a NULL
// market (legacy data written before the schema added the column) fall back
// to defaultMarket. Withdrawn trades (command=1) are excluded from the
// aggregate; legacy rows where command IS NULL are included on the
// assumption they are matched (the same assumption the old aggregator made
// before the column existed).
func (r *QuestDBRepository) FetchDailyBars(
	ctx context.Context,
	startWIB, endWIBExclusive time.Time,
	defaultMarket string,
) ([]domain.DailyBar, error) {
	if !endWIBExclusive.After(startWIB) {
		return nil, fmt.Errorf("end %s must be after start %s",
			endWIBExclusive, startWIB)
	}

	// Convert WIB wall-clock bounds to UTC for QuestDB filter.
	startUTC := startWIB.UTC()
	endUTC := endWIBExclusive.UTC()

	// QuestDB doesn't accept parameter binding on /exec — quote literals.
	// Format with microsecond precision (QuestDB's TIMESTAMP unit).
	const ts = "2006-01-02T15:04:05.000000Z"
	query := fmt.Sprintf(`
SELECT %[1]s,
       stock,
       market,
       first(price)         open,
       max(price)           high,
       min(price)           low,
       last(price)          close,
       sum(volume)          volume,
       sum(price * volume)  value,
       count()              freq
FROM %[2]s
WHERE %[1]s >= '%[3]s' AND %[1]s < '%[4]s'
  AND (command = 0 OR command IS NULL)
SAMPLE BY 1d ALIGN TO CALENDAR TIME ZONE 'Asia/Jakarta'`,
		r.cfg.TsColumn,
		r.cfg.Table,
		startUTC.Format(ts),
		endUTC.Format(ts),
	)

	resp, err := r.exec(ctx, query)
	if err != nil {
		return nil, err
	}

	bars := make([]domain.DailyBar, 0, len(resp.Dataset))
	for i, row := range resp.Dataset {
		bar, err := parseRow(row, defaultMarket)
		if err != nil {
			return nil, fmt.Errorf("parse row %d: %w", i, err)
		}
		bars = append(bars, bar)
	}
	return bars, nil
}

// execResponse is the subset of QuestDB's /exec JSON shape we care about.
//
// Field positions follow the SELECT order:
//
//	0: ts (string ISO-8601), 1: stock, 2..6: ohlc, 6: volume, 7: value, 8: freq
type execResponse struct {
	Query   string          `json:"query"`
	Columns []execColumn    `json:"columns"`
	Dataset [][]any         `json:"dataset"`
	Count   int             `json:"count"`
	Error   string          `json:"error"`
	Position json.RawMessage `json:"position"`
}

type execColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (r *QuestDBRepository) exec(ctx context.Context, query string) (*execResponse, error) {
	u, err := url.Parse(r.cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid questdb base url: %w", err)
	}
	u.Path = "/exec"
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if r.cfg.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.BearerToken)
	} else if r.cfg.BasicAuthUser != "" {
		req.SetBasicAuth(r.cfg.BasicAuthUser, r.cfg.BasicAuthPassword)
	}

	httpResp, err := r.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("questdb http: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("questdb read body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("questdb http %d: %s",
			httpResp.StatusCode, truncate(string(body), 500))
	}

	var resp execResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("questdb decode: %w (body=%s)",
			err, truncate(string(body), 500))
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("questdb query error: %s", resp.Error)
	}
	return &resp, nil
}

// parseRow turns a [ts, stock, market, open, high, low, close, volume, value,
// freq] row into a DailyBar. QuestDB returns numeric fields as JSON numbers
// and timestamps as ISO-8601 strings.
//
// market column may be JSON null for legacy rows written before the schema
// added it — defaultMarket is used as fallback in that case.
func parseRow(row []any, defaultMarket string) (domain.DailyBar, error) {
	if len(row) < 10 {
		return domain.DailyBar{}, fmt.Errorf("expected 10 cols, got %d", len(row))
	}

	tsStr, ok := row[0].(string)
	if !ok {
		return domain.DailyBar{}, fmt.Errorf("col 0 ts: not a string (%T)", row[0])
	}
	ts, err := parseQuestDBTime(tsStr)
	if err != nil {
		return domain.DailyBar{}, fmt.Errorf("parse ts %q: %w", tsStr, err)
	}
	// SAMPLE BY 1d ALIGN TO CALENDAR TIME ZONE 'Asia/Jakarta' returns the
	// bucket start in UTC pointing at WIB midnight. Convert back to WIB
	// then strip time-of-day to get the trading-day DATE.
	wib := ts.In(jakartaLoc)
	date := time.Date(wib.Year(), wib.Month(), wib.Day(), 0, 0, 0, 0, time.UTC)

	stock, ok := row[1].(string)
	if !ok {
		return domain.DailyBar{}, fmt.Errorf("col 1 stock: not a string (%T)", row[1])
	}

	market := defaultMarket
	switch m := row[2].(type) {
	case string:
		if m != "" {
			market = m
		}
	case nil:
		// legacy row — keep defaultMarket
	default:
		return domain.DailyBar{}, fmt.Errorf("col 2 market: unexpected type %T", row[2])
	}

	open, err := jsonInt(row[3])
	if err != nil {
		return domain.DailyBar{}, fmt.Errorf("open: %w", err)
	}
	high, err := jsonInt(row[4])
	if err != nil {
		return domain.DailyBar{}, fmt.Errorf("high: %w", err)
	}
	low, err := jsonInt(row[5])
	if err != nil {
		return domain.DailyBar{}, fmt.Errorf("low: %w", err)
	}
	closePx, err := jsonInt(row[6])
	if err != nil {
		return domain.DailyBar{}, fmt.Errorf("close: %w", err)
	}
	volume, err := jsonInt(row[7])
	if err != nil {
		return domain.DailyBar{}, fmt.Errorf("volume: %w", err)
	}
	value, err := jsonInt(row[8])
	if err != nil {
		return domain.DailyBar{}, fmt.Errorf("value: %w", err)
	}
	freq, err := jsonInt(row[9])
	if err != nil {
		return domain.DailyBar{}, fmt.Errorf("freq: %w", err)
	}

	return domain.DailyBar{
		StockCode: stock,
		Date:      date,
		Market:    market,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     closePx,
		Volume:    volume,
		Value:     value,
		Freq:      freq,
	}, nil
}

// jsonInt accepts both JSON numbers (float64) and strings, returns int64.
// QuestDB sends LONG/DOUBLE as JSON numbers; we round to int because the
// Postgres schema stores OHLC/volume/value as INT/BIGINT.
func jsonInt(v any) (int64, error) {
	switch x := v.(type) {
	case float64:
		// Banker's-rounding-free conversion: round half to even is fine
		// for IDX whole-rupiah prices (integer in float).
		return int64(x + 0.5), nil
	case string:
		// QuestDB sometimes returns large LONGs as strings — fall back.
		n, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return 0, err
		}
		return n, nil
	case nil:
		return 0, errors.New("null value")
	default:
		return 0, fmt.Errorf("unexpected json type %T", v)
	}
}

// parseQuestDBTime parses the ISO-8601 timestamps QuestDB returns. It tries
// a couple of common formats — QuestDB's exact precision varies by version.
func parseQuestDBTime(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05.000000Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("no matching format")
}

// jakartaLoc is the IDX trading timezone — same fixed offset used everywhere
// else in the codebase (no DST in Jakarta).
var jakartaLoc = time.FixedZone("WIB", 7*3600)

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
