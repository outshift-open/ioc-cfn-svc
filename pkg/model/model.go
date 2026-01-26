package model

import "gorm.io/gorm"

type FooType struct {
	gorm.Model
	UUID  string
	Name  string
	Email string
}

type CreateFooRequest struct {
	UUID  string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UserType struct {
	IDPUserID string
	IDPIssuer string
	UUID      string
	Name      string
	Email     string
}

type SessionType struct {
	UserID      string
	AccessToken string
	IDToken     string
}
