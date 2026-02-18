package model

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
