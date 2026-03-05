package audit

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ResourceType enum values
const (
	ResourceTypeCognitionEngine = "COGNITION_ENGINE"
	ResourceTypePolicyEnforcer  = "POLICY_ENFORCER"
	ResourceTypeMemoryProvider  = "MEMORY_PROVIDER"
	ResourceTypeMAS             = "MAS"
	ResourceTypeMASAgent        = "MAS-AGENT"
	ResourceTypeWorkflow        = "WORKFLOW"
	ResourceTypeTask            = "TASK"
)

// AuditType enum values
const (
	AuditTypeResourceCreated    = "RESOURCE_CREATED"
	AuditTypeResourceUpdated    = "RESOURCE_UPDATED"
	AuditTypeResourceDeleted    = "RESOURCE_DELETED"
	AuditTypeResourcePurged     = "RESOURCE_PURGED"
	AuditTypeResourcePruned     = "RESOURCE_PRUNED"
	AuditTypeKnowledgeIngestion = "KNOWLEDGE_INGESTION"
	AuditTypeKnowledgeQuery     = "KNOWLEDGE_QUERY"
	AuditTypeMemoryOperation    = "MEMORY_OPERATION"
)

var validResourceTypes = map[string]bool{
	ResourceTypeCognitionEngine: true,
	ResourceTypePolicyEnforcer:  true,
	ResourceTypeMemoryProvider:  true,
	ResourceTypeMAS:             true,
	ResourceTypeMASAgent:        true,
	ResourceTypeWorkflow:        true,
	ResourceTypeTask:            true,
}

var validAuditTypes = map[string]bool{
	AuditTypeResourceCreated:    true,
	AuditTypeResourceUpdated:    true,
	AuditTypeResourceDeleted:    true,
	AuditTypeResourcePurged:     true,
	AuditTypeResourcePruned:     true,
	AuditTypeKnowledgeIngestion: true,
	AuditTypeKnowledgeQuery:     true,
	AuditTypeMemoryOperation:    true,
}

// IsValidResourceType returns true if the given resource type is a known valid value.
func IsValidResourceType(rt string) bool {
	return validResourceTypes[rt]
}

// IsValidAuditType returns true if the given audit type is a known valid value.
func IsValidAuditType(at string) bool {
	return validAuditTypes[at]
}

// ValidResourceTypesList returns a comma-separated string of all valid resource types.
func ValidResourceTypesList() string {
	keys := make([]string, 0, len(validResourceTypes))
	for k := range validResourceTypes {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

// ValidAuditTypesList returns a comma-separated string of all valid audit types.
func ValidAuditTypesList() string {
	keys := make([]string, 0, len(validAuditTypes))
	for k := range validAuditTypes {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

// ValidateResourceType returns an error if the given resource type is not valid.
func ValidateResourceType(rt string) error {
	if !IsValidResourceType(rt) {
		return fmt.Errorf("invalid resource_type: %s. Valid values: %s", rt, ValidResourceTypesList())
	}
	return nil
}

// ValidateAuditType returns an error if the given audit type is not valid.
func ValidateAuditType(at string) error {
	if !IsValidAuditType(at) {
		return fmt.Errorf("invalid audit_type: %s. Valid values: %s", at, ValidAuditTypesList())
	}
	return nil
}

// Audit represents an immutable audit trail event.
type Audit struct {
	ID                      uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OperationID             *string        `gorm:"size:128" json:"operation_id,omitempty"`
	ResourceType            string         `gorm:"size:64;not null" json:"resource_type"`
	ResourceIdentifier      string         `gorm:"size:128;not null" json:"resource_identifier"`
	AuditType               string         `gorm:"size:64;not null" json:"audit_type"`
	AuditResourceIdentifier string         `gorm:"size:128;not null" json:"audit_resource_identifier"`
	AuditInformation        datatypes.JSON `gorm:"type:jsonb" json:"audit_information,omitempty"`
	AuditExtraInformation   *string        `json:"audit_extra_information,omitempty"`
	CreatedBy               uuid.UUID      `gorm:"type:uuid;not null" json:"created_by"`
	CreatedOn               time.Time      `gorm:"not null" json:"created_on"`
	LastModifiedBy          uuid.UUID      `gorm:"type:uuid;not null" json:"last_modified_by"`
	LastModifiedOn          time.Time      `gorm:"not null" json:"last_modified_on"`
}

// CreateAuditEventRequest is the JSON body for creating an audit event.
type CreateAuditEventRequest struct {
	OperationID             *string        `json:"operation_id,omitempty"`
	ResourceType            string         `json:"resource_type"`
	ResourceIdentifier      string         `json:"resource_identifier"`
	AuditType               string         `json:"audit_type"`
	AuditResourceIdentifier string         `json:"audit_resource_identifier"`
	AuditInformation        datatypes.JSON `json:"audit_information,omitempty"`
	AuditExtraInformation   *string        `json:"audit_extra_information,omitempty"`
	CreatedBy               uuid.UUID      `json:"created_by"`
	LastModifiedBy          uuid.UUID      `json:"last_modified_by"`
}

// MigrateUp runs GORM AutoMigrate for the Audit table.
func MigrateUp(db *gorm.DB) error {
	return db.AutoMigrate(&Audit{})
}

// CreateAuditEvent inserts a new audit event.
func CreateAuditEvent(db *gorm.DB, a *Audit) error {
	if err := ValidateResourceType(a.ResourceType); err != nil {
		return err
	}
	if err := ValidateAuditType(a.AuditType); err != nil {
		return err
	}
	a.ID = uuid.New()
	now := time.Now()
	a.CreatedOn = now
	a.LastModifiedOn = now
	return db.Create(a).Error
}

// GetAuditEventByID retrieves a single audit event by its UUID.
func GetAuditEventByID(db *gorm.DB, id uuid.UUID) (*Audit, error) {
	var a Audit
	if err := db.Where("id = ?", id).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// ListAuditEvents returns audit events with optional resource_type and audit_type filters.
func ListAuditEvents(db *gorm.DB, resourceType, auditType string) ([]Audit, error) {
	var audits []Audit
	query := db.Model(&Audit{})
	if resourceType != "" {
		if err := ValidateResourceType(resourceType); err != nil {
			return nil, err
		}
		query = query.Where("resource_type = ?", resourceType)
	}
	if auditType != "" {
		if err := ValidateAuditType(auditType); err != nil {
			return nil, err
		}
		query = query.Where("audit_type = ?", auditType)
	}
	if err := query.Order("created_on DESC").Find(&audits).Error; err != nil {
		return nil, err
	}
	return audits, nil
}

// DeleteAuditEventByID deletes a single audit event by its UUID. Internal API only.
// Returns an error if the event does not exist.
func DeleteAuditEventByID(db *gorm.DB, id uuid.UUID) error {
	result := db.Where("id = ?", id).Delete(&Audit{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("audit event [%s] not found", id)
	}
	return nil
}
