package web

type EventType int

const (
	EventConfigChanged EventType = iota
	EventRunNow
)

type Event struct {
	Type EventType
}
