package types

import (
	"time"

	"github.com/google/uuid"
)

type MessageId = uint64
type Size64 = uint64
type StreamUUID = uuid.UUID
type StreamIteratorUUID = uuid.UUID
type StreamProperties = map[string]interface{}
type StreamUUIDList []StreamUUID

// Defered stream record to be saved
type DeferedStreamRecord struct {
	Id           MessageId   `json:"i"`
	CreationDate time.Time   `json:"d"`
	Msg          interface{} `json:"m"`
}
