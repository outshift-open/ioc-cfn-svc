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

type Database struct {
	*gorm.DB
}

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

	return &Database{DB: gdb}, nil
}

func (db *Database) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (db *Database) Ping() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func (db *Database) MigrateUp() error {
	if err := audit.MigrateUp(db.DB); err != nil {
		return err
	}
	return nil
}

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

func (db *Database) Create_User(u *model.UserType) error {
	q := db.DB.Create(u)
	if q.Error != nil {
		return q.Error
	}
	return nil
}

func (db *Database) Create_Session(s *model.SessionType) error {
	q := db.DB.Create(s)
	if q.Error != nil {
		return q.Error
	}
	return nil
}

func (db *Database) CreateAuditEvent(a *audit.Audit) error {
	return audit.CreateAuditEvent(db.DB, a)
}

func (db *Database) GetAuditEventByID(id uuid.UUID) (*audit.Audit, error) {
	return audit.GetAuditEventByID(db.DB, id)
}

func (db *Database) ListAuditEvents(resourceType, auditType string) ([]audit.Audit, error) {
	return audit.ListAuditEvents(db.DB, resourceType, auditType)
}

func (db *Database) DeleteAuditEventByID(id uuid.UUID) error {
	return audit.DeleteAuditEventByID(db.DB, id)
}
