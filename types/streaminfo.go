package types

import (
	"time"

	"github.com/itchyny/gojq"
)

type StreamInfo struct {
	UUID         StreamUUID       `json:"uuid" example:"4ce589e2-b483-467b-8b59-758b339801db"`
	CptMessages  Size64           `json:"cptMessages" example:"12345"`
	SizeInBytes  Size64           `json:"sizeInBytes" example:"4567890"`
	CreationDate time.Time        `json:"creationDate"`
	LastUpdate   time.Time        `json:"lastUpdate"`
	Properties   StreamProperties `json:"properties"`
	LastMsgId    MessageId        `json:"lastMsgId"`
}

type StreamInfoList []*StreamInfo

func NewStreamInfo(uuid StreamUUID) *StreamInfo {
	return &StreamInfo{
		UUID:         uuid,
		CreationDate: time.Now(),
		LastUpdate:   time.Now(),
		LastMsgId:    0,
		CptMessages:  0,
		SizeInBytes:  0,
		Properties:   StreamProperties{},
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
