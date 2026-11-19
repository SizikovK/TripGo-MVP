# План выборки устаревших поездок

Проверено 19 ноября 2026 на отдельной схеме с теми же миграциями.
5000 поездок: 100 устаревших активных, 2400 свежих активных, 2500 завершённых.
Перед проверкой выполнен `ANALYZE trips`. Параметр `$1` — 60 секунд.

```sql
SELECT id, driver_id FROM trips
WHERE status = 'active'
  AND COALESCE(last_position_at, started_at) < now() - $1 * interval '1 second'
ORDER BY COALESCE(last_position_at, started_at), id;
```

```text
Sort  (cost=193.81..195.89 rows=833 width=40) (actual time=0.088..0.093 rows=100 loops=1)
  Sort Key: (COALESCE(last_position_at, started_at)), id
  Sort Method: quicksort  Memory: 31kB
  Buffers: shared hit=4
  ->  Bitmap Heap Scan on trips  (cost=34.74..153.40 rows=833 width=40) (actual time=0.034..0.051 rows=100 loops=1)
        Recheck Cond: ((COALESCE(last_position_at, started_at) < (now() - '00:01:00'::interval)) AND (status = 'active'::text))
        Heap Blocks: exact=2
        Buffers: shared hit=4
        ->  Bitmap Index Scan on trips_active_position_at_idx  (cost=0.00..34.53 rows=833 width=0) (actual time=0.026..0.026 rows=100 loops=1)
              Index Cond: (COALESCE(last_position_at, started_at) < (now() - '00:01:00'::interval))
              Buffers: shared hit=2
Planning:
  Buffers: shared hit=71
Planning Time: 0.493 ms
Execution Time: 0.207 ms
```

Выборка использует `trips_active_position_at_idx`. Полного сканирования таблицы нет.
Индекс создаётся отдельной миграцией с `CONCURRENTLY`, вне транзакции.
Тестовая схема после проверки удаляется.
