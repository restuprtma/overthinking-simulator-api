```sql
SELECT
  to_str(to_timezone(timestamp, 'Asia/Jakarta'), 'HH:mm:ss') AS time,
  CASE
    WHEN stock LIKE '%-RNG' THEN replace(stock, '-RNG', '-R')
    WHEN stock LIKE '%-RTN' THEN replace(stock, '-RTN', '-R')
    WHEN stock LIKE '%NG'   THEN left(stock, length(stock) - 2)
    WHEN stock LIKE '%TN'   THEN left(stock, length(stock) - 2)
    ELSE stock
  END AS code,
  CASE
    WHEN stock LIKE '%-RNG' OR stock LIKE '%NG' THEN 'NG'
    WHEN stock LIKE '%-RTN' OR stock LIKE '%TN' THEN 'TN'
    ELSE 'RG'
  END AS market,
  price,
  volume / 100 AS lot,
  CASE
    WHEN buyer_order_no > seller_order_no THEN 'Buy'
    WHEN seller_order_no > buyer_order_no THEN 'Sell'
    ELSE 'Cross'
  END AS action,
  buyer_order_no,
  seller_order_no
FROM trades
WHERE stock IN ('ENRG', 'ENRGNG', 'ENRGTN')
ORDER BY timestamp DESC
LIMIT 100;

```
