package metric

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

var (
	l    *zap.SugaredLogger
	once sync.Once
)

func getLogger() *zap.SugaredLogger {
	once.Do(func() {
		l = logger.SubPkg("metric")
	})
	return l
}

// MASMetric represents a single metric data point in the mas_metrics table.
// This table tracks token usage, operation timing, and other metrics for MAS operations.
// Note: Units are encoded in the metric_name (e.g., llm.operation.duration_ms)
type MASMetric struct {
	Time        time.Time      `gorm:"primaryKey;not null"`
	WorkspaceID uuid.UUID      `gorm:"type:uuid;primaryKey;not null;index:idx_mas_metrics_workspace"`
	MASID       uuid.UUID      `gorm:"type:uuid;primaryKey;not null;index:idx_mas_metrics_mas"`
	AgentID     string         `gorm:"type:text;primaryKey;not null"`
	MetricName  string         `gorm:"type:text;primaryKey;not null"`
	CEID        *uuid.UUID     `gorm:"type:uuid;index:idx_mas_metrics_ce_id"`
	Value       float64        `gorm:"type:double precision;not null"`
	Attributes  datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`
}

// TableName specifies the table name for GORM.
func (MASMetric) TableName() string {
	return "mas_metrics"
}

// MigrateUp creates the mas_metrics table and indexes.
func MigrateUp(db *gorm.DB) error {
	log := getLogger()

	// Create table with GORM
	if err := db.AutoMigrate(&MASMetric{}); err != nil {
		return err
	}

	// Create additional indexes not handled by GORM tags
	// Index for MAS-level queries (optional - primary key may be sufficient)
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_mas_metrics_mas_time
		ON mas_metrics (mas_id, time)
	`).Error; err != nil {
		return err
	}

	// Ensure TimescaleDB extension is installed
	log.Info("Attempting to enable TimescaleDB extension...")
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE`).Error; err != nil {
		log.Errorf("Failed to create TimescaleDB extension: %v", err)
		return err
	}

	// Verify TimescaleDB extension and get version
	var version string
	if err := db.Raw(`SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'`).Scan(&version).Error; err != nil {
		log.Error("TimescaleDB extension not found after installation attempt")
		return err
	}
	log.Infof("TimescaleDB extension installed successfully, version: %s", version)

	// TimescaleDB is available - convert to hypertable if not already
	// Check if table is already a hypertable
	var count int64
	db.Raw(`
		SELECT COUNT(*)
		FROM timescaledb_information.hypertables
		WHERE hypertable_name = 'mas_metrics'
	`).Scan(&count)

	if count == 0 {
		log.Info("Converting mas_metrics to TimescaleDB hypertable...")
		// Convert to hypertable (7-day chunks)
		if err := db.Exec(`
			SELECT create_hypertable('mas_metrics', by_range('time', INTERVAL '7 days'))
		`).Error; err != nil {
			log.Errorf("Failed to create hypertable: %v", err)
			return err
		}
		log.Info("Successfully created hypertable with 7-day chunks")

		// Enable compression (compress chunks older than 7 days)
		// Note: Compression requires TimescaleDB Community edition (Apache license doesn't support it)
		log.Info("Attempting to enable compression for mas_metrics...")
		if err := db.Exec(`
			ALTER TABLE mas_metrics SET (
				timescaledb.compress,
				timescaledb.compress_segmentby = 'workspace_id,mas_id,agent_id,metric_name',
				timescaledb.compress_orderby = 'time DESC'
			)
		`).Error; err != nil {
			log.Warnf("Compression not available (requires TimescaleDB Community edition): %v", err)
			// Don't fail - compression is optional
		} else {
			if err := db.Exec(`
				SELECT add_compression_policy('mas_metrics', INTERVAL '7 days')
			`).Error; err != nil {
				log.Warnf("Compression policy not available (requires TimescaleDB Community edition): %v", err)
			} else {
				log.Info("Successfully enabled compression (7-day policy)")
			}
		}

		// Retention policy (drop chunks older than 90 days)
		log.Info("Attempting to add retention policy (90 days)...")
		if err := db.Exec(`
			SELECT add_retention_policy('mas_metrics', INTERVAL '90 days')
		`).Error; err != nil {
			log.Warnf("Retention policy not available (requires TimescaleDB Community edition): %v", err)
			// Don't fail - retention is optional
		} else {
			log.Info("Successfully added retention policy")
		}
	} else {
		log.Info("mas_metrics is already a hypertable, skipping conversion")
	}

	return nil
}
