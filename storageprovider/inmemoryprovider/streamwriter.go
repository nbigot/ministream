package inmemoryprovider

import (
	"fmt"
	"sync"

	"github.com/nbigot/ministream/types"

	"go.uber.org/zap"
)

type StreamWriterInMemory struct {
	// implements IStreamWriter
	logger         *zap.Logger
	logVerbosity   int
	info           *types.StreamInfo
	inMemoryStream *InMemoryStream
	mu             sync.Mutex
}

func (w *StreamWriterInMemory) Init() error {
	if w.logVerbosity > 1 {
		w.logger.Debug(
			"init stream writer",
			zap.String("topic", "stream"),
			zap.String("method", "Init"),
			zap.String("stream.uuid", w.info.UUID.String()),
		)
	}

	return nil
}

func (w *StreamWriterInMemory) Open() error {
	return nil
}

func (w *StreamWriterInMemory) Close() error {
	return nil
}

func (w *StreamWriterInMemory) Write(records *[]types.DeferedStreamRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(*records) == 0 {
		return nil
	}

	if w.logVerbosity > 0 {
		w.logger.Debug(
			"write records into memory",
			zap.String("topic", "stream"),
			zap.String("method", "Write"),
			zap.String("stream.uuid", w.info.UUID.String()),
			zap.Int("records.cpt", len(*records)),
		)
	}

	if w.info.ReadableMessages.CptMessages == 0 {
		// first message ever of the stream
		w.info.ReadableMessages.FirstMsgId = w.info.IngestedMessages.FirstMsgId
		w.info.ReadableMessages.LastMsgId = 0
		w.info.ReadableMessages.FirstMsgTimestamp = (*records)[0].CreationDate
	}

	// process all records of the ingest buffer
	for _, record := range *records {
		if w.logVerbosity > 1 {
			w.logger.Debug(
				"write record into memory",
				zap.String("topic", "stream"),
				zap.String("method", "Write"),
				zap.String("stream.uuid", w.info.UUID.String()),
				zap.Any("msg", record),
			)
		}

		// append the record to data memory
		if err := w.inMemoryStream.AddRecord(&record); err != nil {
			return err
		}

		// update info
		w.info.ReadableMessages.CptMessages += 1
		w.info.ReadableMessages.LastMsgTimestamp = record.CreationDate
		w.info.ReadableMessages.SizeInBytes += uint64(len(fmt.Sprintf("%v", record.Msg)))
		w.info.ReadableMessages.LastMsgId = record.Id
	}

	return nil
}

func NewStreamWriterInMemory(info *types.StreamInfo, inMemoryStream *InMemoryStream, logger *zap.Logger, logVerbosity int) *StreamWriterInMemory {
	return &StreamWriterInMemory{
		logger:         logger,
		logVerbosity:   logVerbosity,
		info:           info,
		inMemoryStream: inMemoryStream,
	}
}
