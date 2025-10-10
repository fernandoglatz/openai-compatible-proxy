package exceptions

var (
	GenericError = BaseError{
		Code:    "GENERIC_ERROR",
		Message: "Try again later.",
	}
	RecordNotFound = BaseError{
		Code:    "RECORD_NOT_FOUND",
		Message: "Record not found.",
	}
	HeaderNotFound = BaseError{
		Code:    "HEADER_NOT_FOUND",
		Message: "Header not found.",
	}
	InvalidJSON = BaseError{
		Code:    "INVALID_JSON",
		Message: "Invalid JSON.",
	}
)

type WrappedError struct {
	Error     error
	Message   string
	Code      string
	BaseError BaseError
}

type BaseError struct {
	Code    string
	Message string
}

type ApiError struct {
	Message string
	Status  int
}

func (wrappedError WrappedError) GetMessage() string {
	if wrappedError.Error != nil {
		return wrappedError.Error.Error()
	}

	if wrappedError.Message != "" {
		return wrappedError.Message
	}

	if wrappedError.BaseError.Message != "" {
		return wrappedError.BaseError.Message
	}

	return ""
}

func (wrappedError WrappedError) GetCode() string {
	if wrappedError.Code != "" {
		return wrappedError.Code
	}

	if wrappedError.BaseError.Code != "" {
		return wrappedError.BaseError.Code
	}

	return ""
}
