package models

type Chat struct {
	ID           int64
	ActiveSmoke  bool
	SmokeSession *SmokeSession
}
