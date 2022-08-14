package rbac

const ActionGetRecords = "GetRecords"
const ActionCreateRecordsIterator = "CreateRecordsIterator"
const ActionPutRecords = "PutRecords"
const ActionPutRecord = "PutRecord"
const ActionGetRecordsIteratorStats = "ActionGetRecordsIteratorStats"
const ActionListStreams = "ListStreams"
const ActionListStreamsProperties = "ListStreamsProperties"
const ActionGetStreamDescription = "GetStreamDescription"
const ActionGetStreamProperties = "GetStreamProperties"
const ActionSetStreamProperties = "SetStreamProperties"
const ActionUpdateStreamProperties = "UpdateStreamProperties"
const ActionGetStreamRawFile = "GetStreamRawFile"
const ActionCreateStream = "CreateStream"
const ActionDeleteStream = "DeleteStream"
const ActionCloseRecordsIterator = "CloseRecordsIterator"
const ActionRebuildIndex = "RebuildIndex"
const ActionListUsers = "ListUsers"
const ActionValidateApiKey = "ValidateApiKey"
const ActionGetAccount = "GetAccount"
const ActionStopServer = "StopServer"
const ActionReloadServerAuth = "ReloadServerAuth"
const ActionJWTRevokeAll = "ActionJWTRevokeAll"

var ActionList = []string{
	ActionGetRecords, ActionCreateRecordsIterator, ActionPutRecords, ActionPutRecord, ActionGetRecordsIteratorStats,
	ActionListStreams, ActionListStreamsProperties, ActionGetStreamDescription, ActionGetStreamProperties,
	ActionSetStreamProperties, ActionUpdateStreamProperties, ActionGetStreamRawFile, ActionCreateStream, ActionDeleteStream,
	ActionCloseRecordsIterator, ActionRebuildIndex, ActionListUsers,
	ActionValidateApiKey, ActionGetAccount, ActionStopServer, ActionReloadServerAuth, ActionJWTRevokeAll,
}
