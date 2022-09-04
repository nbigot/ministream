package jsonfileprovider

import (
	"ministream/types"
	"os"

	"go.uber.org/zap"
)

type StreamWriterFile struct {
	// implements IStreamWriter
	logger    *zap.Logger
	info      *types.StreamInfo
	fileData  *os.File
	fileIndex *os.File
}

func (w *StreamWriterFile) Write(record *[]types.DeferedStreamRecord) {
	//TODO ICI: il faut appeler la méthode qui save les data
	// !! voir FileStorage::saveIngestBuffer
	// besoin de:
	//	 ingestBuffer *buffering.StreamIngestBuffer
	//			--> lock/unlock du mutex --> inutile car déjà fait dans cette fonction s.mu.Lock()
	//			--> range ingestBuffer.GetBuffer() --> à remplacer par paramètre &s.msgBuffer (eq. *[]DeferedStreamRecord)
	//      --> ingestBuffer.Clear() --> inutile car déjà fait dans cette fonction
	//	 info *types.StreamInfo --> pourrait être directement dans l'interface cible
	//	 fileData *os.File
	//	 fileIndex *os.File
	panic("not implemented")
}

func NewStreamWriterFile(info *types.StreamInfo, fileDataPath string, fileIndexPath string, logger *zap.Logger) (*StreamWriterFile, error) {
	// func (s *Stream) GetIndex() *StreamIndexFile {
	// 	if s.index == nil {
	// 		s.index = NewStreamIndex(s.info.UUID, s.logger)
	// 	}
	// 	return s.index
	// }

	//panic("pas terminé")
	// conflit avec FileStorage::SaveStream

	streamUUID := info.UUID.String()

	// Open stream data file
	fileData, err := os.OpenFile(fileDataPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error(
			"can't open data file",
			zap.String("topic", "stream"),
			zap.String("method", "save"),
			zap.String("stream.uuid", streamUUID),
			zap.Any("filename", fileDataPath),
			zap.Error(err),
		)
		return nil, err
	}
	//defer fileData.Close()

	// Open stream index file
	fileIndex, err := os.OpenFile(fileIndexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error(
			"can't open index file",
			zap.String("topic", "stream"),
			zap.String("method", "save"),
			zap.String("stream.uuid", streamUUID),
			zap.Any("filename", fileIndexPath),
			zap.Error(err),
		)
		return nil, err
	}
	//defer fileIndex.Close()

	return &StreamWriterFile{
		logger:    logger,
		info:      info,
		fileData:  fileData,
		fileIndex: fileIndex,
	}, nil
}
