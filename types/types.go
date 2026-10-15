package types

import (
	"math"
	"math/big"
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

// NewRecordValue returns a record as generic JSON values {"i": id, "d": date, "m": message}
// that jq filters can process: gojq only accepts nil, bool, int, float64, *big.Int, string,
// []interface{} and map[string]interface{} values (no go structs, time.Time nor uint64),
// and it panics on other values. The message must be made of such values (decoded JSON).
func NewRecordValue(id MessageId, creationDate time.Time, msg interface{}) map[string]interface{} {
	var jqId interface{}
	if id <= math.MaxInt {
		jqId = int(id)
	} else {
		jqId = new(big.Int).SetUint64(id)
	}
	return map[string]interface{}{
		"i": jqId,
		"d": creationDate.Format(time.RFC3339Nano),
		"m": msg,
	}
}
