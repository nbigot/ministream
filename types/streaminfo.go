package types

import (
	"time"

	"github.com/itchyny/gojq"
)

type StreamMessagesInfo struct {
	CptMessages       Size64    `json:"cptMessages" example:"12345"`
	SizeInBytes       Size64    `json:"sizeInBytes" example:"4567890"`
	FirstMsgId        MessageId `json:"firstMsgId"`
	LastMsgId         MessageId `json:"lastMsgId"`
	FirstMsgTimestamp time.Time `json:"firstMsgTimestamp"`
	LastMsgTimestamp  time.Time `json:"fastMsgTimestamp"`
}

type StreamInfo struct {
	UUID             StreamUUID         `json:"uuid" example:"4ce589e2-b483-467b-8b59-758b339801db"`
	CreationDate     time.Time          `json:"creationDate"`
	LastUpdate       time.Time          `json:"lastUpdate"`
	Properties       StreamProperties   `json:"properties"`
	IngestedMessages StreamMessagesInfo `json:"ingestedMessages"` // messages that have been ingested in the stream
	ReadableMessages StreamMessagesInfo `json:"readableMessages"` // messages that are readable by a consumer
}

type StreamInfoList []*StreamInfo

type StreamInfoDict map[StreamUUID]*StreamInfo

func NewStreamInfo(uuid StreamUUID) *StreamInfo {
	now := time.Now()
	return &StreamInfo{
		UUID:         uuid,
		CreationDate: now,
		LastUpdate:   now,
		Properties:   StreamProperties{},
		IngestedMessages: StreamMessagesInfo{
			CptMessages: 0,
			SizeInBytes: 0,
			FirstMsgId:  0,
			LastMsgId:   0,
		},
		ReadableMessages: StreamMessagesInfo{
			CptMessages: 0,
			SizeInBytes: 0,
			FirstMsgId:  0,
			LastMsgId:   0,
		},
	}
}

func (s *StreamInfo) UpdateProperties(properties *StreamProperties) {
	// add or update properties
	if properties != nil {
		for key, value := range *properties {
			s.Properties[key] = value
		}
	}
}

func (s *StreamInfo) SetProperties(properties *StreamProperties) {
	// delete all existing properties
	for k := range s.Properties {
		delete(s.Properties, k)
	}

	// add new properties
	if properties != nil {
		for key, value := range *properties {
			s.Properties[key] = value
		}
	}
}

func (s *StreamInfo) MatchFilterProperties(jqFilter *gojq.Query) (bool, error) {
	jqIter := jqFilter.Run(s.Properties)
	for {
		v, ok := jqIter.Next()
		if !ok {
			return false, nil
		}

		if err, ok := v.(error); ok {
			return false, err
		}

		// if v is true then stream is matching the filter
		if v == true {
			// stream properties are matching the jq filter
			return true, nil
		} else {
			// stream properties does not match the jq filter
			return false, nil
		}
	}
}
