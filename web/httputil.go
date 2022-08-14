package web

import (
	"ministream/stream"
	"time"
)

type JSONResultSuccess struct {
	Code    int    `json:"code" example:"200"`
	Message string `json:"message" example:"success"`
}

type JSONResult struct {
	Code    int         `json:"code" example:"200"`
	Message string      `json:"message" example:"success"`
	Data    interface{} `json:"data"`
}

type HTTPError struct {
	Code    int    `json:"code" example:"400"`
	Message string `json:"message" example:"error"`
}

type JSONResultListStreamsProperties struct {
	Code   int                                    `json:"code" example:"200"`
	Result *JSONResultListStreamsPropertiesResult `json:"result"`
}

type JSONResultListStreamsPropertiesResult struct {
	Total int                                         `json:"total" example:"5"`
	Rows  *[]JSONResultListStreamsPropertiesResultRow `json:"rows"`
}

type JSONResultListStreamsPropertiesResultRow struct {
	FilePath     string                  `json:"filepath"`
	UUID         stream.StreamUUID       `json:"uuid" example:"4ce589e2-b483-467b-8b59-758b339801db"`
	CptMessages  stream.Size64           `json:"cptMessages" example:"12345"`
	SizeInBytes  stream.Size64           `json:"sizeInBytes" example:"4567890"`
	CreationDate time.Time               `json:"creationDate"`
	LastUpdate   time.Time               `json:"lastUpdate"`
	Properties   stream.StreamProperties `json:"properties"`
	LastMsgId    stream.MessageId        `json:"lastMsgId"`
}
