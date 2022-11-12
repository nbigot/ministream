package inmemoryprovider

import (
	"errors"
	"fmt"
	"ministream/types"
	"sync"
	"time"
)

type InMemoryStream struct {
	streamUUID         types.StreamUUID
	info               *types.StreamInfo
	records            []*InMemoryRecord
	mu                 sync.Mutex
	maxRecordsByStream uint64
	maxSizeInBytes     uint64
}

type InMemoryRecord struct {
	Id           types.MessageId `json:"i"`
	CreationDate time.Time       `json:"d"`
	Msg          interface{}     `json:"m"`
}

func (s *InMemoryStream) AddRecord(record *types.DeferedStreamRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.maxRecordsByStream > 0 && uint64(len(s.records)) >= s.maxRecordsByStream {
		return fmt.Errorf("cannot add more record into the stream (limit is %d)", s.maxRecordsByStream)
	}

	// TODO: check maxSizeInBytes

	// append the record to data memory
	inMemoryRecord := InMemoryRecord{Id: record.Id, CreationDate: record.CreationDate, Msg: record.Msg}
	s.records = append(s.records, &inMemoryRecord)

	return nil
}

func (s *InMemoryStream) GetRecordAtIndex(index uint64) (*InMemoryRecord, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch cptRecords := uint64(len(s.records)); {
	case index+1 == cptRecords:
		// result is: (record, record found, cannot continue because this is the last record)
		return s.records[index], true, false
	case index < cptRecords:
		// result is: (record, record found, can continue because there are one or many records after this one)
		return s.records[index], true, true
	default:
		// result is: (no record, no record found, cannot continue)
		return nil, false, false
	}
}

func (s *InMemoryStream) GetRecordsCount() uint64 {
	return uint64(len(s.records))
}

func (s *InMemoryStream) GetIndexAtMessageId(messageId types.MessageId) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cptRecords := uint64(len(s.records))
	if cptRecords == 0 {
		return 0, errors.New("no matching record not found")
	}

	return s.searchRecordIndexByRecordId(messageId, cptRecords-1)
}

func (s *InMemoryStream) GetIndexAfterMessageId(messageId types.MessageId) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cptRecords := uint64(len(s.records))
	if cptRecords == 0 {
		return 0, errors.New("no matching record not found")
	}

	if recordIndex, err := s.searchRecordIndexByRecordId(messageId, cptRecords-1); err != nil {
		return recordIndex, err
	} else {
		return recordIndex + 1, err
	}
}

func (s *InMemoryStream) GetIndexAtTimestamp(timestamp *time.Time) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cptRecords := uint64(len(s.records))
	if cptRecords == 0 {
		return 0, errors.New("no matching record not found")
	}

	timestampUnixNano := timestamp.UnixNano()
	return s.searchRecordIndexAtOrAfterTimestamp(timestampUnixNano, cptRecords-1)
}

func (s *InMemoryStream) searchRecordIndexByRecordId(messageId types.MessageId, lastIndexRank uint64) (uint64, error) {
	// use a dichotomy algorithm to find the index rank for the given MessageId
	// if no result found then return an error "no matching record not found"
	// assume every message has a unique id
	// assume message id values are always increasing as the index rank increase
	var lowIndexRank uint64 = 0
	var highIndexRank uint64 = lastIndexRank
	var nextMedianIndexRank uint64
	var record *InMemoryRecord

	if len(s.records) == 0 {
		return 0, errors.New("no matching record not found")
	}

	for lowIndexRank < highIndexRank {
		nextMedianIndexRank = (lowIndexRank + highIndexRank) >> 1
		record = s.records[nextMedianIndexRank]
		if record.Id == messageId {
			// record id was found
			return nextMedianIndexRank, nil
		}
		if record.Id > messageId {
			highIndexRank = nextMedianIndexRank
		} else {
			lowIndexRank = nextMedianIndexRank + 1
		}
	}

	record = s.records[nextMedianIndexRank]
	if record.Id == messageId {
		// record id was found
		return nextMedianIndexRank, nil
	}

	// can't find record id
	return 0, errors.New("no matching record not found")
}

func (s *InMemoryStream) searchRecordIndexAtOrAfterTimestamp(timestampUnixNano int64, lastIndexRank uint64) (uint64, error) {
	// use a dichotomy algorithm to find the index rank for the given timestamp
	// if no result found then return an error "no matching record not found"
	// assume every message has a unique id
	// assume message id values are always increasing as the index rank increase
	var lowIndexRank uint64 = 0
	var highIndexRank uint64 = lastIndexRank
	var nextMedianIndexRank uint64
	var recordTimestampUnixNano int64
	var record *InMemoryRecord

	if len(s.records) == 0 {
		return 0, errors.New("no matching record not found")
	}

	for lowIndexRank < highIndexRank {
		nextMedianIndexRank = (lowIndexRank + highIndexRank) >> 1
		record = s.records[nextMedianIndexRank]
		recordTimestampUnixNano = record.CreationDate.UnixNano()
		if recordTimestampUnixNano == timestampUnixNano {
			// exact timestamp was found
			return nextMedianIndexRank, nil
		}
		if recordTimestampUnixNano > timestampUnixNano {
			highIndexRank = nextMedianIndexRank
		} else {
			lowIndexRank = nextMedianIndexRank + 1
		}
	}

	record = s.records[nextMedianIndexRank]
	recordTimestampUnixNano = record.CreationDate.UnixNano()
	if recordTimestampUnixNano >= timestampUnixNano {
		// found a message that was created after the given timestamp
		return nextMedianIndexRank, nil
	}

	// can't find message created after timestamp
	return 0, errors.New("no matching record not found")
}

func NewInMemoryStream(info *types.StreamInfo, maxRecordsByStream uint64, maxSizeInBytes uint64) (*InMemoryStream, error) {
	return &InMemoryStream{
		info:               info,
		streamUUID:         info.UUID,
		records:            make([]*InMemoryRecord, 0),
		maxRecordsByStream: maxRecordsByStream,
		maxSizeInBytes:     maxSizeInBytes,
	}, nil
}
