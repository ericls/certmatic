package endpoint

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type apiResponse struct {
	Data   any        `json:"data"`
	Errors []apiError `json:"errors"`
}

type apiError struct {
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func decodeRequestBodyJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	bodyStrBuilder := new(strings.Builder)
	if _, err := io.Copy(bodyStrBuilder, r.Body); err != nil {
		return err
	}
	bodyStr := bodyStrBuilder.String()
	if bodyStr == "" {
		bodyStr = "{}"
	}
	if err := json.Unmarshal([]byte(bodyStr), v); err != nil {
		return err
	}
	return validate.Struct(v)
}

func writeJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(apiResponse{Data: data, Errors: []apiError{}})
}

func writeError(w http.ResponseWriter, status int, message string) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(apiResponse{
		Data:   nil,
		Errors: []apiError{{Message: message}},
	})
}

func writeValidationErrors(w http.ResponseWriter, errs validator.ValidationErrors) error {
	apiErrors := make([]apiError, 0, len(errs))
	for _, e := range errs {
		apiErrors = append(apiErrors, apiError{Field: e.Field(), Message: e.Tag()})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	return json.NewEncoder(w).Encode(apiResponse{Data: nil, Errors: apiErrors})
}

type JSONHandlerFunc[TReq any, TRes any] func(r *http.Request, body TReq) (TRes, error)

type HTTPError struct {
	Status  int
	Message string
}

func (e HTTPError) Error() string { return e.Message }

func JSONHandler[TReq, TRes any](statusOnOK int, handler JSONHandlerFunc[TReq, TRes]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		shouldRequestHaveBody := method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch
		var body TReq
		if shouldRequestHaveBody {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<19) // 512KB
			if err := decodeRequestBodyJSON(r, &body); err != nil {
				var validationErrs validator.ValidationErrors
				if errors.As(err, &validationErrs) {
					writeValidationErrors(w, validationErrs)
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
