package gateway

import (
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

// apiError is one error answer: its envelope and the status it is sent with.
type apiError struct {
	api.Error
	Status int
}

var (
	errorUnauthenticated             = admissionError(auth.AdmissionUnauthenticated)
	errorInvalidRequest              = admissionError(auth.AdmissionInvalid)
	errorRequestTooLarge             = admissionError(auth.AdmissionTooLarge)
	errorWorkingDirectoryInvalid     = apiError{api.Error{Code: "WorkingDirectoryInvalid", Message: "Choose a valid working directory."}, http.StatusUnprocessableEntity}
	errorWorkingDirectoryUnavailable = apiError{api.Error{Code: "WorkingDirectoryUnavailable", Message: "That directory does not exist or cannot be opened."}, http.StatusUnprocessableEntity}
	errorDirectoryListingUnavailable = apiError{api.Error{Code: "DirectoryListingUnavailable", Message: "This directory cannot be browsed. Enter the path instead."}, http.StatusUnprocessableEntity}
	errorDirectoryListingTooLarge    = apiError{api.Error{Code: "DirectoryListingTooLarge", Message: "This directory has too many folders to show. Enter the path instead."}, http.StatusUnprocessableEntity}
	errorDirectorySearchUnavailable  = apiError{api.Error{Code: "DirectorySearchUnavailable", Message: "Directory search is unavailable on this machine."}, http.StatusUnprocessableEntity}
	errorDirectorySearchTooLarge     = apiError{api.Error{Code: "DirectorySearchTooLarge", Message: "Too many directory search results. Narrow the search."}, http.StatusUnprocessableEntity}
	errorTerminalContextUnavailable  = apiError{api.Error{Code: "TerminalContextUnavailable", Message: "Remote context is unavailable."}, http.StatusNotFound}
	errorProfileUnknown              = apiError{api.Error{Code: "ProfileUnknown", Message: "Choose an available profile."}, http.StatusUnprocessableEntity}
	errorSessionNameInvalid          = apiError{api.Error{Code: "SessionNameInvalid", Message: "use 1–64 letters, numbers, underscores, or hyphens; start with a letter or number."}, http.StatusUnprocessableEntity}
	errorSessionNameChanged          = apiError{api.Error{Code: "SessionNameChanged", Message: "the session name changed. review and save again."}, http.StatusConflict}
	errorSessionNameConflict         = apiError{api.Error{Code: "SessionNameConflict", Message: "another session on this machine uses that name."}, http.StatusConflict}
	errorObjectiveInvalid            = apiError{api.Error{Code: "ObjectiveInvalid", Message: "Use 1–240 characters without terminal controls."}, http.StatusUnprocessableEntity}
	errorGroupInvalid                = apiError{api.Error{Code: "GroupInvalid", Message: group.ErrInvalid.Error()}, http.StatusUnprocessableEntity}
	errorSessionNotFound             = apiError{api.Error{Code: "SessionNotFound", Message: "That session no longer exists."}, http.StatusNotFound}
	errorSessionIdentityMismatch     = apiError{api.Error{Code: "SessionIdentityMismatch", Message: "The session changed. Refresh and try again."}, http.StatusConflict}
	errorMachineIdentityMismatch     = admissionError(auth.AdmissionMachineMismatch)
	errorInternal                    = admissionError(auth.AdmissionUnavailable)
)

func admissionError(code auth.AdmissionCode) apiError {
	status, message := code.Response()
	return apiError{api.Error{Code: string(code), Message: message}, status}
}
