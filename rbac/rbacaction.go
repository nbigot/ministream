package rbac

const ActionGetRecords = "GetRecords"
const ActionCreateRecordsIterator = "CreateRecordsIterator"
const ActionPutRecords = "PutRecords"
const ActionPutRecord = "PutRecord"
const ActionGetRecordsIteratorStats = "GetRecordsIteratorStats"
const ActionListStreams = "ListStreams"
const ActionListStreamsProperties = "ListStreamsProperties"
const ActionGetStreamDescription = "GetStreamDescription"
const ActionGetStreamProperties = "GetStreamProperties"
const ActionSetStreamProperties = "SetStreamProperties"
const ActionUpdateStreamProperties = "UpdateStreamProperties"
const ActionCreateStream = "CreateStream"
const ActionDeleteStream = "DeleteStream"
const ActionCloseRecordsIterator = "CloseRecordsIterator"
const ActionRebuildIndex = "RebuildIndex"
const ActionListUsers = "ListUsers"
const ActionGetAccount = "GetAccount"
const ActionShutdownServer = "ShutdownServer"
const ActionRestartServer = "RestartServer"
const ActionJWTRevokeAll = "JWTRevokeAll"

var ActionList = []string{
	ActionGetRecords, ActionCreateRecordsIterator, ActionPutRecords, ActionPutRecord, ActionGetRecordsIteratorStats,
	ActionListStreams, ActionListStreamsProperties, ActionGetStreamDescription, ActionGetStreamProperties,
	ActionSetStreamProperties, ActionUpdateStreamProperties, ActionCreateStream, ActionDeleteStream,
	ActionCloseRecordsIterator, ActionRebuildIndex, ActionListUsers,
	ActionGetAccount, ActionShutdownServer, ActionRestartServer, ActionJWTRevokeAll,
}
