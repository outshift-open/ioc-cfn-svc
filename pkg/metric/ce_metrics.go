package metric

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CEMetric represents a single metric data point in the ce_metrics table.
// This table tracks Cognition Engine infrastructure metrics (service-level).
// Examples: inference latency, queue depth, model token usage, request throughput.
// Note: Units are encoded in the metric_name (e.g., ce.inference.latency_ms)
type CEMetric struct {
	Time       time.Time      `gorm:"primaryKey;not null"`
	CEID       uuid.UUID      `gorm:"type:uuid;primaryKey;not null;index:idx_ce_metrics_lookup"`
	MetricName string         `gorm:"type:text;primaryKey;not null"`
	Value      float64        `gorm:"type:double precision;not null"`
	Attributes datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`
}

// TableName specifies the table name for GORM.
func (CEMetric) TableName() string {
	return "ce_metrics"
}

// MigrateCEMetricsUp creates the ce_metrics table and indexes.
func MigrateCEMetricsUp(db *gorm.DB) error {
	log := getLogger()

	// Create table with GORM
	if err := db.AutoMigrate(&CEMetric{}); err != nil {
		return err
	}

	// Create additional indexes not handled by GORM tags
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_ce_metrics_name
		ON ce_metrics (ce_id, metric_name, time DESC)
	`).Error; err != nil {
		return err
	}

	// Check if table is already a hypertable
	var count int64
	db.Raw(`
		SELECT COUNT(*)
		FROM timescaledb_information.hypertables
		WHERE hypertable_name = 'ce_metrics'
	`).Scan(&count)

	if count == 0 {
		log.Info("Converting ce_metrics to TimescaleDB hypertable...")
		// Convert to hypertable (7-day chunks)
		if err := db.Exec(`
			SELECT create_hypertable('ce_metrics', by_range('time', INTERVAL '7 days'))
		`).Error; err != nil {
			log.Errorf("Failed to create hypertable: %v", err)
			return err
		}
		log.Info("Successfully created hypertable with 7-day chunks")

		// Add space partitioning by ce_id (4 partitions)
		log.Info("Adding space partitioning by ce_id...")
		if err := db.Exec(`
			SELECT add_dimension('ce_metrics', by_hash('ce_id', 4))
		`).Error; err != nil {
			log.Warnf("Space partitioning not available: %v", err)
			// Don't fail - space partitioning is optional
		}

		// Enable compression (compress chunks older than 7 days)
		log.Info("Attempting to enable compression for ce_metrics...")
		if err := db.Exec(`
			ALTER TABLE ce_metrics SET (
				timescaledb.compress,
				timescaledb.compress_segmentby = 'ce_id,metric_name',
				timescaledb.compress_orderby = 'time DESC'
			)
		`).Error; err != nil {
			log.Warnf("Compression not available (requires TimescaleDB Community edition): %v", err)
			// Don't fail - compression is optional
		} else {
			if err := db.Exec(`
				SELECT add_compression_policy('ce_metrics', INTERVAL '7 days')
			`).Error; err != nil {
				log.Warnf("Compression policy not available (requires TimescaleDB Community edition): %v", err)
			} else {
				log.Info("Successfully enabled compression (7-day policy)")
			}
		}

		// Retention policy (drop chunks older than 90 days)
		log.Info("Attempting to add retention policy (90 days)...")
		if err := db.Exec(`
			SELECT add_retention_policy('ce_metrics', INTERVAL '90 days')
		`).Error; err != nil {
			log.Warnf("Retention policy not available (requires TimescaleDB Community edition): %v", err)
			// Don't fail - retention is optional
		} else {
			log.Info("Successfully added retention policy")
		}
	} else {
		log.Info("ce_metrics is already a hypertable, skipping conversion")
	}

	return nil
}
