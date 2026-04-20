package database

import (
	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/prometheus"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/audit"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/config"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/model"
)

// Database wraps a GORM DB connection to PostgreSQL.
type Database struct {
	*gorm.DB
}

// New opens a PostgreSQL connection using the provided config and registers Prometheus metrics.
func New(cfg config.Database) (*Database, error) {
	dsn := cfg.DSN()

	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, errors.New(err)
	}

	err = gdb.Use(prometheus.New(prometheus.Config{
		DBName:          cfg.Name,
		RefreshInterval: 15,
		StartServer:     false,
	}))
	if err != nil {
		return nil, errors.New(err)
	}

	// TODO: Configure connection pool settings (MaxOpenConns, MaxIdleConns, ConnMaxLifetime, ConnMaxIdleTime) to avoid default GORM/database/sql pool defaults if needed.
	return &Database{DB: gdb}, nil
}

// Close closes the underlying database connection.
func (db *Database) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Ping verifies the database connection is alive.
func (db *Database) Ping() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// MigrateUp runs all auto-migrations (audit tables, etc.).
func (db *Database) MigrateUp() error {
	if err := audit.MigrateUp(db.DB); err != nil {
		return err
	}
	return nil
}

// Find_User_By_IDPUserID_And_Issuer looks up a user by IDP user ID and issuer.
func (db *Database) Find_User_By_IDPUserID_And_Issuer(idpUserID string,
	idpIssuer string) (*model.UserType, error) {

	var user model.UserType
	q := db.DB.
		Where(&model.UserType{
			IDPUserID: idpUserID,
			IDPIssuer: idpIssuer,
		}).Find(&user)
	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, nil
	}

	return &user, nil
}

// Create_User inserts a new user record.
func (db *Database) Create_User(u *model.UserType) error {
	q := db.DB.Create(u)
	if q.Error != nil {
		return q.Error
	}
	return nil
}

// Create_Session inserts a new session record.
func (db *Database) Create_Session(s *model.SessionType) error {
	q := db.DB.Create(s)
	if q.Error != nil {
		return q.Error
	}
	return nil
}

// CreateAuditEvent inserts a new audit event.
func (db *Database) CreateAuditEvent(a *audit.Audit) error {
	return audit.CreateAuditEvent(db.DB, a)
}

// GetAuditEventByID retrieves a single audit event by UUID.
func (db *Database) GetAuditEventByID(id uuid.UUID) (*audit.Audit, error) {
	return audit.GetAuditEventByID(db.DB, id)
}

// ListAuditEvents returns audit events with optional resource_type and audit_type filters.
func (db *Database) ListAuditEvents(resourceType, auditType string, page, pageSize int) (*audit.AuditListResponse, error) {
	return audit.ListAuditEvents(db.DB, resourceType, auditType, page, pageSize)
}

// DeleteAuditEventByID deletes a single audit event by UUID.
func (db *Database) DeleteAuditEventByID(id uuid.UUID) error {
	return audit.DeleteAuditEventByID(db.DB, id)
}
