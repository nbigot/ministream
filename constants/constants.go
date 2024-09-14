package constants

// Errors

const ErrorInvalidStreamUuid = 1000
const ErrorStreamUuidNotFound = 1001
const ErrorCantCreateStream = 1002
const ErrorCantDeleteStream = 1003
const ErrorInvalidJQFilter = 1004
const ErrorInvalidIteratorUuid = 1005
const ErrorInvalidParameterValue = 1006
const ErrorCantDeserializeJson = 1007
const ErrorDuplicatedBatchId = 1008

const ErrorCantPutMessageIntoStream = 1010
const ErrorCantPutMessagesIntoStream = 1011
const ErrorCantDeserializeJsonRecords = 1012
const ErrorInvalidCreateRecordsIteratorRequest = 1013
const ErrorCantCreateRecordsIterator = 1014

const ErrorCantGetMessagesFromStream = 1020

const ErrorCantCloseStreamIterator = 1030
const ErrorStreamIteratorNotFound = 1031
const ErrorStreamIteratorIsBusy = 1032

const ErrorCantRebuildStreamIndex = 1040

const ErrorInvalidJobUuid = 1100
const ErrorJobUuidNotFound = 1101
const ErrorCantCreateJob = 1102

const ErrorJWTMissingOrMalformed = 1200
const ErrorJWTInvalidOrExpired = 1201
const ErrorJWTNotEnabled = 1202
const ErrorJWTRBACUnknownRole = 1210
const ErrorRBACInvalidRule = 1211
const ErrorRBACForbidden = 1212
const ErrorRBACNotEnabled = 1213
const ErrorAuthInternalError = 1220
const ErrorWrongCredentials = 1230

// Misc

const JWTClaimsAccountKey = "account"
const JWTClaimsUserKey = "user"
const JWTClaimsRolesKey = "roles"
const JWTContextKey = "jwt"

const SuperUserContextKey = "superuser"
const UserContextKey = "user"
const RolesContextKey = "roles"
const RBACContextKey = "rbac"
const ABACContextKey = "abac"

const APIKEY = "API-KEY"

// Auth hash methods

const AuthHashMethodNone = 0   // plain text
const AuthHashMethodMD5 = 1    // MD5
const AuthHashMethodSHA256 = 2 // SHA256
const AuthHashMethodSHA512 = 3 // SHA512
const AuthHashMethodSHA1 = 4   // SHA1

// Http params

const ParamNameStreamIteratorUuid = "streamiteratoruuid"
