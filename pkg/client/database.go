package client

import (
	"sync"

	"github.com/go-errors/errors"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/database"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/model"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

var log = logger.SubPkg("app")

type Database interface {
	Close() error
	Ping() error
	MigrateUp() error

	Create_Foo(*model.FooType) error
	Get_Foo_By_UUID(string) (*model.FooType, error)

	Find_User_By_IDPUserID_And_Issuer(string, string) (*model.UserType, error)
	Create_User(*model.UserType) error
	Create_Session(*model.SessionType) error
}

// ensure at build time that this mock type fulfills the interface
var _ Database = (*MockDatabase)(nil)
var _ Database = (*database.Database)(nil)

type MockDatabase struct {
	mockFooStore      map[string]*model.FooType
	fooStoreMutex     *sync.Mutex
	mockUserStore     map[string]*model.UserType
	userStoreMutex    *sync.Mutex
	mockSessionStore  map[string]*model.SessionType
	sessionStoreMutex *sync.Mutex
}

func NewMockDatabase() *MockDatabase {
	log.Warn("creating mock [database] service")
	return &MockDatabase{
		mockFooStore:      make(map[string]*model.FooType),
		fooStoreMutex:     &sync.Mutex{},
		mockUserStore:     make(map[string]*model.UserType),
		userStoreMutex:    &sync.Mutex{},
		mockSessionStore:  make(map[string]*model.SessionType),
		sessionStoreMutex: &sync.Mutex{},
	}
}

func (m *MockDatabase) Close() error {
	return nil
}

func (m *MockDatabase) Ping() error {
	return errors.New("mocked service - not connected to real service")
}

func (m *MockDatabase) MigrateUp() error {
	return nil
}

func (m *MockDatabase) Create_Foo(f *model.FooType) error {
	m.fooStoreMutex.Lock()
	defer m.fooStoreMutex.Unlock()
	m.mockFooStore[f.UUID] = f
	return nil
}

func (m *MockDatabase) Get_Foo_By_UUID(uuid string) (*model.FooType, error) {
	m.fooStoreMutex.Lock()
	defer m.fooStoreMutex.Unlock()
	foo, exists := m.mockFooStore[uuid]
	if !exists {
		return nil, errors.Errorf("[%s] not found", uuid)
	}
	return foo, nil
}

func (m *MockDatabase) Find_User_By_IDPUserID_And_Issuer(idpUserID string,
	idpIssuer string) (*model.UserType, error) {

	m.userStoreMutex.Lock()
	defer m.userStoreMutex.Unlock()

	for _, u := range m.mockUserStore {
		if u.IDPUserID == idpUserID && u.IDPIssuer == idpIssuer {
			return u, nil
		}
	}
	return nil, nil
}

func (m *MockDatabase) Create_User(u *model.UserType) error {
	m.userStoreMutex.Lock()
	defer m.userStoreMutex.Unlock()
	m.mockUserStore[u.Email] = u
	return nil
}

func (m *MockDatabase) Create_Session(s *model.SessionType) error {
	m.sessionStoreMutex.Lock()
	defer m.sessionStoreMutex.Unlock()
	m.mockSessionStore[s.AccessToken] = s
	return nil
}
