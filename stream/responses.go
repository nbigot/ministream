package stream

import (
	. "github.com/nbigot/ministream/types"
)

type GetStreamRecordsResponse struct {
	Status             string             `json:"status"`
	Duration           int64              `json:"duration"`
	Count              int64              `json:"count"`
	CountErrors        int64              `json:"countErrors"`
	CountSkipped       int64              `json:"countSkipped"`
	Remain             bool               `json:"remain"`
	LastRecordIdRead   MessageId          `json:"lastRecordIdRead"`
	StreamUUID         StreamUUID         `json:"streamUUID"`
	StreamIteratorUUID StreamIteratorUUID `json:"streamIteratorUUID"`
	Records            []interface{}      `json:"records"`
}

type CreateRecordsIteratorResponse struct {
	Status             string             `json:"status"`
	Message            string             `json:"message"`
	StreamUUID         StreamUUID         `json:"streamUUID"`
	StreamIteratorUUID StreamIteratorUUID `json:"streamIteratorUUID"`
}

type CloseRecordsIteratorResponse struct {
	Status             string             `json:"status"`
	Message            string             `json:"message"`
	StreamUUID         StreamUUID         `json:"streamUUID"`
	StreamIteratorUUID StreamIteratorUUID `json:"streamIteratorUUID"`
}

type GetRecordsIteratorStatsResponse struct {
	Status             string             `json:"status"`
	Message            string             `json:"message"`
	StreamUUID         StreamUUID         `json:"streamUUID"`
	StreamIteratorUUID StreamIteratorUUID `json:"streamIteratorUUID"`
	LastRecordIdRead   MessageId          `json:"lastRecordIdRead"`
	Name               string             `json:"name"`
}

type PutStreamRecordsResponse struct {
	Status     string      `json:"status"`
	StreamUUID StreamUUID  `json:"streamUUID"`
	Duration   int64       `json:"duration"`
	Count      int64       `json:"count"`
	MessageIds []MessageId `json:"messageIds"`
}

type LoginAccountResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	JWT     string `json:"jwt"`
}

type LoginUserResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	JWT     string `json:"jwt"`
}

type RebuildStreamIndexResponse struct {
	Status     string      `json:"status"`
	Message    string      `json:"message"`
	StreamUUID StreamUUID  `json:"streamUUID"`
	Duration   int64       `json:"duration"`
	IndexStats interface{} `json:"indexStats"`
}
