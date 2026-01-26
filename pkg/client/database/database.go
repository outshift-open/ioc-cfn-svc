package database

import (
	"github.com/go-errors/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/prometheus"

	"github.com/cisco-eti/sre-go-helloworld/pkg/config"
	"github.com/cisco-eti/sre-go-helloworld/pkg/model"
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
	err := db.DB.AutoMigrate(&model.FooType{})
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) Create_Foo(f *model.FooType) error {
	res := db.DB.Create(f)
	return errors.New(res.Error)
}

func (db *Database) Get_Foo_By_UUID(uuid string) (*model.FooType, error) {
	foo := model.FooType{}
	res := db.DB.Where("uuid = ?", uuid).Find(&foo)
	if res.Error != nil {
		return nil, errors.New(res.Error)
	}
	return &foo, nil
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
