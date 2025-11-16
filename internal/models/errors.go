package models

const (
	ErrMsgInvalidRequestBody    = "invalid request body"
	ErrMsgInternalServerError   = "internal server error"
	ErrMsgResourceNotFound      = "resource not found"
	ErrMsgTeamNameRequired      = "team_name is required"
	ErrMsgUserIDRequired        = "user_id is required"
	ErrMsgTeamNameAlreadyExists = "team_name already exists"
	ErrMsgPRIDAlreadyExists     = "PR id already exists"
	ErrMsgCannotReassignMerged  = "cannot reassign on merged PR"
	ErrMsgPRInvalidStatus       = "PR cannot be merged with current status"
	ErrMsgReviewerNotAssigned   = "reviewer is not assigned to this PR"
	ErrMsgNoCandidate           = "no active replacement candidate in team"
	ErrMsgStringTooLong         = "string exceeds maximum length"
	ErrMsgUsernameRequired      = "username is required"
	ErrMsgDuplicateUserInTeam   = "duplicate user ID in team"
)

type ErrorCode string

const (
	ErrorCodeTeamExists          ErrorCode = "TEAM_EXISTS"
	ErrorCodePRExists            ErrorCode = "PR_EXISTS"
	ErrorCodePRMerged            ErrorCode = "PR_MERGED"
	ErrorCodeNotAssigned         ErrorCode = "NOT_ASSIGNED"
	ErrorCodeNoCandidate         ErrorCode = "NO_CANDIDATE"
	ErrorCodeNotFound            ErrorCode = "NOT_FOUND"
	ErrorCodeValidationError     ErrorCode = "VALIDATION_ERROR"
	ErrorCodeInternalServerError ErrorCode = "INTERNAL_SERVER_ERROR"
	ErrorCodeMaxReviewers        ErrorCode = "MAX_REVIEWERS"
)

type ErrorResponse struct {
	Error struct {
		Code    ErrorCode `json:"code"`
		Message string    `json:"message"`
	} `json:"error"`
}

func NewErrorResponse(code ErrorCode, message string) *ErrorResponse {
	resp := &ErrorResponse{}
	resp.Error.Code = code
	resp.Error.Message = message
	return resp
}
