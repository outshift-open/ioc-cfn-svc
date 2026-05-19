# TimescaleDB Setup Guide

## Quick Reference

TimescaleDB is **always enabled** for metrics storage. The service requires PostgreSQL with TimescaleDB extension installed.

## Requirements

- PostgreSQL 12+ with TimescaleDB extension
- Use TimescaleDB Docker image: `timescale/timescaledb:latest-pg16`
- Or install TimescaleDB: https://docs.timescale.com/install/

## What Happens During Migration

The migration (`pkg/metric/mas_metrics.go`) automatically:

1. **Install Extension**: `CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE`
2. **Convert to Hypertable**: 7-day chunks for efficient time-series queries
3. **Enable Compression**: Compress chunks older than 7 days
4. **Set Retention**: Auto-drop chunks older than 90 days

## Verification

### Check Application Logs

On startup, you'll see:
```
INFO: Attempting to enable TimescaleDB extension...
INFO: TimescaleDB extension installed successfully, version: 2.14.2
INFO: Converting mas_metrics to TimescaleDB hypertable...
INFO: Successfully created hypertable with 7-day chunks
INFO: Attempting to enable compression for mas_metrics...
WARN: Compression not available (requires TimescaleDB Community edition)
INFO: Attempting to add retention policy (90 days)...
WARN: Retention policy not available (requires TimescaleDB Community edition)
```

**Note**: Compression and retention policies require TimescaleDB Community edition. The Apache license version works but without these features.

### Check Database

Connect to PostgreSQL and run:

```sql
-- Verify extension is installed
SELECT extversion FROM pg_extension WHERE extname = 'timescaledb';

-- Verify hypertable exists
SELECT * FROM timescaledb_information.hypertables WHERE hypertable_name = 'mas_metrics';

-- Check compression settings
SELECT * FROM timescaledb_information.compression_settings WHERE hypertable_name = 'mas_metrics';
```

## Troubleshooting

### Migration Fails: "extension timescaledb does not exist"

**Cause**: TimescaleDB extension not available in PostgreSQL.

**Fix**:
1. Use TimescaleDB Docker image: `timescale/timescaledb:latest-pg16`
2. Or install TimescaleDB: https://docs.timescale.com/install/

### Migration on Existing Table

The migration is **idempotent**:
- If table exists as regular PostgreSQL table, it will be converted to hypertable
- If already a hypertable, migration is skipped
- No data loss during conversion

## Performance Impact

| Feature | Without TimescaleDB | With TimescaleDB |
|---------|---------------------|------------------|
| Writes | Fast | Fast (same) |
| Time-range queries | Slower on large tables | Much faster (partitioning) |
| Storage | Regular | Compressed (50-90% savings) |
| Retention | Manual | Automatic (90-day policy) |

## References

- [TimescaleDB Documentation](https://docs.timescale.com/)
- [Hypertable Design Doc](./timescaledb-hypertables.md)
- [Migration Code](../pkg/metric/mas_metrics.go)
