-- =============================================================================
-- QuestDB Materialized Views — historical OHLCV per timeframe
-- =============================================================================
-- This file is canonical DDL — run it manually against your QuestDB cluster
-- (it isn't picked up by `make migrate-*`, which only handles Postgres).
--
-- Why MAT VIEWs at all:
--   The `running_trades` table is raw ticks (one row per matched trade). Chart
--   loads that ask for "last 90 days × 1m bars × 50 stocks" against `running_trades`
--   force QuestDB to scan ~270M rows and recompute SAMPLE BY each request.
--   A 1m MAT VIEW pre-aggregates this into ~6M rows; query latency drops
--   from seconds to <100ms.
--
-- Required QuestDB version: 8.0+ (earlier versions don't have MAT VIEWs —
-- see fallback at the bottom of this file).
--
-- Verify the new `command` and `market` columns exist on `running_trades` before
-- running — see docs/infra/topology.md §5.3 ALTER TABLE.
--
--   curl -G 'http://localhost:9000/exec' \
--        --data-urlencode 'query=SHOW COLUMNS FROM running_trades;'
-- =============================================================================


-- -----------------------------------------------------------------------------
-- candles_1m — 1-minute bars derived from the running_trades tick table.
-- Filters out withdrawn trades (command=1). Legacy rows where command IS
-- NULL are treated as matched (consistent with the daily-ingest behaviour).
-- -----------------------------------------------------------------------------
CREATE MATERIALIZED VIEW candles_1m AS (
    SELECT timestamp,
           stock,
           market,
           first(price)         AS open,
           max(price)           AS high,
           min(price)           AS low,
           last(price)          AS close,
           sum(volume)          AS volume,
           sum(price * volume)  AS value,
           count()              AS freq
    FROM running_trades
    WHERE command = 0 OR command IS NULL
    SAMPLE BY 1m
) PARTITION BY DAY;


-- -----------------------------------------------------------------------------
-- candles_5m / 15m / 1h / 4h — derived directly from `running_trades`.
--
-- Why not chain off candles_1m? Some QuestDB releases don't support a
-- MAT VIEW that reads from another MAT VIEW. Keeping all of these on the
-- raw `running_trades` base table is portable across versions; the cost is small
-- because QuestDB scans the same partitions for each (compute is cheap,
-- I/O is shared via OS page cache).
--
-- If your version DOES support chained MAT VIEWs and you want to optimise
-- further, replace `FROM running_trades ...` with `FROM candles_1m` and use
-- first(open)/max(high)/min(low)/last(close)/sum(volume,value,freq).
-- -----------------------------------------------------------------------------
CREATE MATERIALIZED VIEW candles_5m AS (
    SELECT timestamp, stock, market,
           first(price) AS open, max(price) AS high, min(price) AS low, last(price) AS close,
           sum(volume) AS volume, sum(price * volume) AS value, count() AS freq
    FROM running_trades
    WHERE command = 0 OR command IS NULL
    SAMPLE BY 5m
) PARTITION BY DAY;

CREATE MATERIALIZED VIEW candles_15m AS (
    SELECT timestamp, stock, market,
           first(price) AS open, max(price) AS high, min(price) AS low, last(price) AS close,
           sum(volume) AS volume, sum(price * volume) AS value, count() AS freq
    FROM running_trades
    WHERE command = 0 OR command IS NULL
    SAMPLE BY 15m
) PARTITION BY DAY;

CREATE MATERIALIZED VIEW candles_1h AS (
    SELECT timestamp, stock, market,
           first(price) AS open, max(price) AS high, min(price) AS low, last(price) AS close,
           sum(volume) AS volume, sum(price * volume) AS value, count() AS freq
    FROM running_trades
    WHERE command = 0 OR command IS NULL
    SAMPLE BY 1h
) PARTITION BY MONTH;

CREATE MATERIALIZED VIEW candles_4h AS (
    SELECT timestamp, stock, market,
           first(price) AS open, max(price) AS high, min(price) AS low, last(price) AS close,
           sum(volume) AS volume, sum(price * volume) AS value, count() AS freq
    FROM running_trades
    WHERE command = 0 OR command IS NULL
    SAMPLE BY 4h
) PARTITION BY MONTH;


-- =============================================================================
-- Sample queries
-- =============================================================================

-- Last 270 1m bars for one stock (one trading day's worth of intraday data,
-- assuming session length ≈ 270 minutes after lunch break):
--
--   SELECT * FROM candles_1m
--   WHERE stock = 'BBCA' AND market = 'RG'
--   ORDER BY timestamp DESC
--   LIMIT 270;

-- WIB-aligned daily roll-up from 1m bars (use this if you don't want a
-- separate candles_1d MAT VIEW — daily lives in Postgres anyway):
--
--   SELECT timestamp, stock, market,
--          first(open) AS open, max(high) AS high, min(low) AS low, last(close) AS close,
--          sum(volume) AS volume, sum(value) AS value, sum(freq) AS freq
--   FROM candles_1m
--   WHERE stock = 'BBCA' AND timestamp >= dateadd('d', -30, now())
--   SAMPLE BY 1d ALIGN TO CALENDAR TIME ZONE 'Asia/Jakarta';


-- =============================================================================
-- Drop / rebuild
-- =============================================================================
-- DROP MATERIALIZED VIEW candles_1m;     -- and so on for the others


-- =============================================================================
-- Fallback for QuestDB < 8.0 (no MAT VIEW support)
-- =============================================================================
-- Use a regular table + a scheduled INSERT INTO ... SELECT batched outside
-- QuestDB (cron / systemd timer / a small Go binary). Schema mirrors the
-- MAT VIEW projection:
--
--   CREATE TABLE candles_1m (
--       timestamp TIMESTAMP,
--       stock     SYMBOL CAPACITY 2048 INDEX,
--       market    SYMBOL CAPACITY 8,
--       open      DOUBLE,
--       high      DOUBLE,
--       low       DOUBLE,
--       close     DOUBLE,
--       volume    LONG,
--       value     DOUBLE,
--       freq      LONG
--   ) timestamp(timestamp) PARTITION BY DAY
--   WAL DEDUP UPSERT KEYS(timestamp, stock, market);
--
-- Then a periodic backfill job (every 1m) runs:
--
--   INSERT INTO candles_1m
--   SELECT timestamp, stock, market,
--          first(price), max(price), min(price), last(price),
--          sum(volume), sum(price * volume), count()
--   FROM running_trades
--   WHERE command = 0 OR command IS NULL
--     AND timestamp >= dateadd('m', -2, now())   -- small overlap, dedup handles it
--   SAMPLE BY 1m;
