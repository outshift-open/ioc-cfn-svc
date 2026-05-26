package client

import (
	"sync"
	"time"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/audit"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/database"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/model"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/otelreceiver"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

var (
	l    *zap.SugaredLogger
	once sync.Once
)

func getLogger() *zap.SugaredLogger {
	once.Do(func() {
		l = logger.SubPkg("app")
	})
	return l
}

type Database interface {
	Close() error
	Ping() error
	MigrateUp() error

	Find_User_By_IDPUserID_And_Issuer(string, string) (*model.UserType, error)
	Create_User(*model.UserType) error
	Create_Session(*model.SessionType) error

	CreateAuditEvent(*audit.Audit) error
	GetAuditEventByID(uuid.UUID) (*audit.Audit, error)
	ListAuditEvents(resourceType, auditType string, page, pageSize int) (*audit.AuditListResponse, error)
	DeleteAuditEventByID(uuid.UUID) error

	BulkInsertOtelSpans([]otelreceiver.OtelSpan) error

	// Task scheduling
	FindDueTasks() ([]model.Task, error)
	UpsertTask(task *model.Task) error
	UpdateTaskStatus(taskID uint, status string, fields map[string]interface{}) error
	RecoverExpiredCallbacks() (int64, error)
	InsertTaskExecutionHistory(h *model.TaskExecutionHistory) error
	UpdateTaskExecutionHistory(id uint, fields map[string]interface{}) error
	UpdateLatestExecutionHistoryByTaskID(taskID uint, fields map[string]interface{}) error
	FindTaskByKey(workspaceID, masID, taskName string) (*model.Task, error)
}

// ensure at build time that this mock type fulfills the interface
var _ Database = (*MockDatabase)(nil)
var _ Database = (*database.Database)(nil)

type MockDatabase struct {
	mockUserStore     map[string]*model.UserType
	userStoreMutex    *sync.Mutex
	mockSessionStore  map[string]*model.SessionType
	sessionStoreMutex *sync.Mutex
	mockAuditStore    map[uuid.UUID]*audit.Audit
	auditStoreMutex   *sync.Mutex
}

func NewMockDatabase() *MockDatabase {
	log := getLogger()
	log.Warn("creating mock [database] service")
	return &MockDatabase{
		mockUserStore:     make(map[string]*model.UserType),
		userStoreMutex:    &sync.Mutex{},
		mockSessionStore:  make(map[string]*model.SessionType),
		sessionStoreMutex: &sync.Mutex{},
		mockAuditStore:    make(map[uuid.UUID]*audit.Audit),
		auditStoreMutex:   &sync.Mutex{},
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

func (m *MockDatabase) CreateAuditEvent(a *audit.Audit) error {
	if err := audit.ValidateResourceType(a.ResourceType); err != nil {
		return err
	}
	if err := audit.ValidateAuditType(a.AuditType); err != nil {
		return err
	}
	m.auditStoreMutex.Lock()
	defer m.auditStoreMutex.Unlock()
	a.ID = uuid.New()
	now := time.Now()
	a.CreatedOn = now
	a.LastModifiedOn = now
	m.mockAuditStore[a.ID] = a
	return nil
}

func (m *MockDatabase) GetAuditEventByID(id uuid.UUID) (*audit.Audit, error) {
	m.auditStoreMutex.Lock()
	defer m.auditStoreMutex.Unlock()
	a, exists := m.mockAuditStore[id]
	if !exists {
		return nil, errors.Errorf("audit event [%s] not found", id)
	}
	return a, nil
}

func (m *MockDatabase) ListAuditEvents(resourceType, auditType string, page, pageSize int) (*audit.AuditListResponse, error) {
	if resourceType != "" {
		if err := audit.ValidateResourceType(resourceType); err != nil {
			return nil, err
		}
	}
	if auditType != "" {
		if err := audit.ValidateAuditType(auditType); err != nil {
			return nil, err
		}
	}
	if pageSize <= 0 {
		pageSize = audit.DefaultPageSize()
	}
	if pageSize > audit.MaxPageSize() {
		pageSize = audit.MaxPageSize()
	}
	if page < 0 {
		page = 0
	}
	m.auditStoreMutex.Lock()
	defer m.auditStoreMutex.Unlock()
	var filtered []audit.Audit
	for _, a := range m.mockAuditStore {
		if resourceType != "" && a.ResourceType != resourceType {
			continue
		}
		if auditType != "" && a.AuditType != auditType {
			continue
		}
		filtered = append(filtered, *a)
	}
	totalElements := len(filtered)
	offset := page * pageSize
	var result []audit.Audit
	if offset < totalElements {
		end := offset + pageSize
		if end > totalElements {
			end = totalElements
		}
		result = filtered[offset:end]
	} else {
		result = []audit.Audit{}
	}
	return &audit.AuditListResponse{
		Data: result,
		PageInfo: audit.PageInfo{
			Page:          page,
			PageSize:      pageSize,
			PageCount:     len(result),
			TotalElements: totalElements,
		},
	}, nil
}

func (m *MockDatabase) DeleteAuditEventByID(id uuid.UUID) error {
	m.auditStoreMutex.Lock()
	defer m.auditStoreMutex.Unlock()
	if _, exists := m.mockAuditStore[id]; !exists {
		return errors.Errorf("audit event [%s] not found", id)
	}
	delete(m.mockAuditStore, id)
	return nil
}

func (m *MockDatabase) BulkInsertOtelSpans(_ []otelreceiver.OtelSpan) error {
	return nil
}

func (m *MockDatabase) FindDueTasks() ([]model.Task, error) {
	return nil, nil
}

func (m *MockDatabase) UpsertTask(_ *model.Task) error {
	return nil
}

func (m *MockDatabase) UpdateTaskStatus(_ uint, _ string, _ map[string]interface{}) error {
	return nil
}

func (m *MockDatabase) RecoverExpiredCallbacks() (int64, error) {
	return 0, nil
}

func (m *MockDatabase) InsertTaskExecutionHistory(_ *model.TaskExecutionHistory) error {
	return nil
}

func (m *MockDatabase) UpdateTaskExecutionHistory(_ uint, _ map[string]interface{}) error {
	return nil
}

func (m *MockDatabase) UpdateLatestExecutionHistoryByTaskID(_ uint, _ map[string]interface{}) error {
	return nil
}

func (m *MockDatabase) FindTaskByKey(_, _, _ string) (*model.Task, error) {
	return nil, nil
}
