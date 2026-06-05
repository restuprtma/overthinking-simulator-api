-- =============================================================================
-- QuestDB sanity-check queries
-- =============================================================================
-- Run via: curl -G http://10.10.8.51:9000/exec --data-urlencode 'query=...'
-- Or paste into the QuestDB web console (port 9000).
-- =============================================================================


-- 1. Schema check — pastikan 13 kolom + market + command ada
SHOW COLUMNS FROM running_trades;


-- 2. Latest trades for one stock (broker codes harus real, bukan "--")
SELECT timestamp, stock, market, price, volume, buyer, seller, trade_no
FROM running_trades
WHERE stock = 'BBCA'
ORDER BY timestamp DESC
LIMIT 10;


-- 3. DEDUP working — banyak distinct broker code, bukan cuma "--"
--    Threshold sehat: ratusan distinct selama IDX session.
SELECT count_distinct(buyer)  AS unique_buyers,
       count_distinct(seller) AS unique_sellers,
       count()                AS total_rows
FROM running_trades;


-- 4. Schema integrity — pastikan tidak ada NULL di kolom baru
SELECT count() AS total,
       sum(case when market  is null then 1 else 0 end) AS null_market,
       sum(case when command is null then 1 else 0 end) AS null_command,
       sum(case when command = 1     then 1 else 0 end) AS withdrawn
FROM running_trades;


-- 5. Top stocks by turnover today (WIB)
SELECT stock,
       count()              AS trades,
       sum(volume)          AS total_volume,
       sum(price * volume)  AS turnover
FROM running_trades
WHERE command = 0
SAMPLE BY 1d ALIGN TO CALENDAR TIME ZONE 'Asia/Jakarta'
LATEST ON timestamp PARTITION BY stock;


-- 6. MAT VIEW vs raw aggregation — should match exactly
--    Run both, eyeball: same OHLCV, same volume, same trade count.
SELECT timestamp, open, high, low, close, volume, freq
FROM candles_1m
WHERE stock = 'BBCA'
ORDER BY timestamp DESC LIMIT 5;

SELECT timestamp,
       first(price) AS open, max(price) AS high, min(price) AS low,
       last(price)  AS close, sum(volume) AS volume, count() AS freq
FROM running_trades
WHERE stock = 'BBCA' AND command = 0
SAMPLE BY 1m
ORDER BY timestamp DESC LIMIT 5;


-- 7. Per-trading-day count (WIB calendar) — verify trades land in correct day
SELECT timestamp,
       count()        AS trades,
       sum(volume)    AS volume
FROM running_trades
SAMPLE BY 1d ALIGN TO CALENDAR TIME ZONE 'Asia/Jakarta';


-- 8. Foreign vs domestic share for one stock
SELECT timestamp,
       sum(case when buyer_type  = 'F' then volume else 0 end) AS foreign_buy_volume,
       sum(case when seller_type = 'F' then volume else 0 end) AS foreign_sell_volume,
       sum(volume) AS total_volume
FROM running_trades
WHERE stock = 'BBCA' AND command = 0
SAMPLE BY 1d ALIGN TO CALENDAR TIME ZONE 'Asia/Jakarta';


-- 9. Withdrawn trades audit — should be tiny fraction (<1%)
SELECT count() AS withdrawn_total,
       count_distinct(stock) AS stocks_with_withdrawals
FROM running_trades
WHERE command = 1;


-- 10. Per-board volume (kalau pernah ada market != 'RG')
SELECT market,
       count() AS trades,
       sum(volume) AS volume,
       sum(price * volume) AS turnover
FROM running_trades;


-- 11. Last bar for each stock — quick "latest snapshot" board view
SELECT timestamp, stock, close, volume
FROM candles_1m
LATEST ON timestamp PARTITION BY stock
ORDER BY timestamp DESC
LIMIT 50;


-- 12. Hourly progression for a single stock — useful for verifying chart load
SELECT timestamp, open, high, low, close, volume
FROM candles_1h
WHERE stock = 'BBCA'
ORDER BY timestamp DESC
LIMIT 24;
