package models

type SmokeSession struct {
	SmokeStarter      int64
	SmokeMessageID    int64
	JoinedUsers       []string
	OriginalSmokeText string
}
