package endpoint

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return err
	}
	return validate.Struct(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, errorCode string) error {
	return writeJSON(w, status, ErrorResponse{
		ErrorCode: errorCode,
	})
}

type ErrorResponse struct {
	ErrorCode string `json:"error"`
}

type JSONHandlerFunc[TReq any, TRes any] func(r *http.Request, body TReq) (TRes, error)

type HTTPError struct {
	Status  int
	Message string
}

type ValidationErrorResponse struct {
	Errors map[string]string `json:"errors"`
}

func formatValidationErrors(errs validator.ValidationErrors) map[string]string {
	result := make(map[string]string, len(errs))
	for _, e := range errs {
		result[e.Field()] = e.Tag() // e.g. {"Name": "required"}
	}
	return result
}

func (e HTTPError) Error() string { return e.Message }

func JSONHandler[TReq any, TRes any](statusOnOK int, handler JSONHandlerFunc[TReq, TRes]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		shouldRequestHaveBody := method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch
		var body TReq
		if shouldRequestHaveBody {
			if err := decodeJSON(r, &body); err != nil {
				var validationErrs validator.ValidationErrors
				if errors.As(err, &validationErrs) {
					writeJSON(w, http.StatusUnprocessableEntity, ValidationErrorResponse{
						Errors: formatValidationErrors(validationErrs),
					})
					return
				}
				writeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
		}

		res, err := handler(r, body)
		if err != nil {
			var httpErr HTTPError
			if errors.As(err, &httpErr) {
				writeError(w, httpErr.Status, httpErr.Message)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		writeJSON(w, statusOnOK, res)
	}
}
