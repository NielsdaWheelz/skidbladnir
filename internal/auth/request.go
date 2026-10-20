package auth

import (
	"net/http"
	"strings"
)

type AdmissionCode string

const (
	AdmissionInvalid         AdmissionCode = "InvalidRequest"
	AdmissionUnauthenticated AdmissionCode = "Unauthenticated"
	AdmissionUnavailable     AdmissionCode = "InternalError"
	AdmissionMachineMismatch AdmissionCode = "MachineIdentityMismatch"
	AdmissionTooLarge        AdmissionCode = "RequestTooLarge"
)

// Response is the public contract shared by authenticated HTTP services.
func (code AdmissionCode) Response() (status int, message string) {
	switch code {
	case AdmissionInvalid:
		return http.StatusBadRequest, "The request is not valid."
	case AdmissionUnauthenticated:
		return http.StatusUnauthorized, "Authentication required."
	case AdmissionUnavailable:
		return http.StatusInternalServerError, "Skíðblaðnir could not complete the request."
	case AdmissionMachineMismatch:
		return http.StatusConflict, "The machine identity changed. Fleet reset is required."
	case AdmissionTooLarge:
		return http.StatusRequestEntityTooLarge, "The request is too large."
	default:
		panic("unhandled request admission") // justify-defect: callers supply this closed code set.
	}
}

func BindMachine(request *http.Request, handle string) AdmissionCode {
	values := request.Header.Values("Skidbladnir-Machine")
	if len(values) > 1 || len(values) == 1 && strings.ContainsRune(values[0], ',') {
		return AdmissionInvalid
	}
	if len(values) != 1 || values[0] != handle {
		return AdmissionMachineMismatch
	}
	return ""
}
