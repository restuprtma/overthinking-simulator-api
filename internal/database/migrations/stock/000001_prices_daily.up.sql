-- =====================================================
-- STOCK SCHEMA - PRICES DAILY (standalone)
-- =====================================================
-- Migration 001: Create the stock schema + the daily OHLCV table.
--
-- Scope: this is intentionally the only stock migration for now —
-- enough to let cmd/daily-ingest write its output. Other stock tables
-- (sectors, brokers, stocks, intraday, broker_summary, done_transactions)
-- live in QuestDB or are not yet needed in Postgres.
--
-- Design notes:
--   - One row per (stock_code, date, market). `market` is the IDX
--     trading board code (RG=Regular, TN=Tunai/Cash, NG=Negosiasi).
--   - NOT partitioned: daily rows are small (~800 stocks × 3 markets
--     × 252 trading days ≈ 600k rows/year). Fits fine without
--     partitioning for many years.
--   - Integer prices: IDX quotes are whole rupiah; `value` (turnover)
--     is BIGINT because a single day on a large cap can exceed 2^31.
--   - No FK to a stocks table: heavy time-series tables in this codebase
--     skip FKs by convention; integrity is validated at ingest time.
-- =====================================================

CREATE SCHEMA IF NOT EXISTS stock;

COMMENT ON SCHEMA stock IS
    'Market reference & time-series data. Public — not scoped to a company.';

CREATE TABLE IF NOT EXISTS stock.prices_daily (
    stock_code VARCHAR(10) NOT NULL,
    date       DATE        NOT NULL,
    market     VARCHAR(3)  NOT NULL,

    open       INT  NOT NULL,
    high       INT  NOT NULL,
    low        INT  NOT NULL,
    close      INT  NOT NULL,

    volume     BIGINT NOT NULL DEFAULT 0,
    value      BIGINT NOT NULL DEFAULT 0,
    freq       INT    NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (stock_code, date, market),

    CONSTRAINT chk_prices_daily_market
        CHECK (market IN ('RG', 'TN', 'NG')),
    CONSTRAINT chk_prices_daily_ohlc_order
        CHECK (low <= open AND low <= close
               AND high >= open AND high >= close
               AND high >= low),
    CONSTRAINT chk_prices_daily_non_negative
        CHECK (volume >= 0 AND value >= 0 AND freq >= 0)
);

-- Chart query: latest N bars of a stock
CREATE INDEX IF NOT EXISTS idx_prices_daily_stock_date
    ON stock.prices_daily (stock_code, date DESC);

-- Scan-all-stocks-on-date query (e.g. end-of-day snapshots)
CREATE INDEX IF NOT EXISTS idx_prices_daily_date
    ON stock.prices_daily (date DESC);

COMMENT ON TABLE stock.prices_daily IS
    'Daily OHLCV per (stock, date, market). Not partitioned — low volume.';
COMMENT ON COLUMN stock.prices_daily.market IS
    'IDX trading board: RG=Regular, TN=Tunai (cash), NG=Negosiasi (negotiated).';
COMMENT ON COLUMN stock.prices_daily.value IS
    'Total turnover in rupiah (price × volume summed over the day).';
COMMENT ON COLUMN stock.prices_daily.freq IS
    'Number of matched trades on this bar.';
