package endpoint

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

// ---------- writeJSON ----------

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name   string
		status int
		data   any
	}{
		{"struct", http.StatusOK, map[string]string{"key": "value"}},
		{"nil", http.StatusOK, nil},
		{"string", http.StatusCreated, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeJSON(rec, tt.status, tt.data)

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}

			var resp apiResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(resp.Errors) != 0 {
				t.Errorf("expected empty errors, got %+v", resp.Errors)
			}
		})
	}
}

// ---------- writeError ----------

func TestWriteError(t *testing.T) {
	tests := []struct {
		status  int
		message string
	}{
		{http.StatusBadRequest, "bad request"},
		{http.StatusNotFound, "not found"},
		{http.StatusInternalServerError, "internal error"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeError(rec, tt.status, tt.message)

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}

			var resp apiResponse
			json.NewDecoder(rec.Body).Decode(&resp)
			if len(resp.Errors) != 1 || resp.Errors[0].Message != tt.message {
				t.Errorf("errors = %+v, want message %q", resp.Errors, tt.message)
			}
		})
	}
}

// ---------- writeValidationErrors ----------

func TestWriteValidationErrors(t *testing.T) {
	type testStruct struct {
		Name string `validate:"required"`
	}
	err := validate.Struct(testStruct{})
	if err == nil {
		t.Fatal("expected validation error")
	}

	rec := httptest.NewRecorder()
	// validate.Struct returns validator.ValidationErrors directly.
	valErrs := err.(validator.ValidationErrors)
	writeValidationErrors(rec, valErrs)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	var resp apiResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(resp.Errors))
	}
	if resp.Errors[0].Field != "Name" {
		t.Errorf("Field = %q, want %q", resp.Errors[0].Field, "Name")
	}
}

// ---------- decodeRequestBodyJSON ----------

func TestDecodeRequestBodyJSON_Valid(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	body := strings.NewReader(`{"name":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	var p payload
	if err := decodeRequestBodyJSON(req, &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Name != "test" {
		t.Errorf("Name = %q, want %q", p.Name, "test")
	}
}

func TestDecodeRequestBodyJSON_EmptyBody(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	body := strings.NewReader("")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	var p payload
	if err := decodeRequestBodyJSON(req, &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Name != "" {
		t.Errorf("Name = %q, want empty", p.Name)
	}
}

func TestDecodeRequestBodyJSON_InvalidJSON(t *testing.T) {
	body := strings.NewReader("{invalid}")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	var p struct{ Name string }
	if err := decodeRequestBodyJSON(req, &p); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDecodeRequestBodyJSON_ValidationFail(t *testing.T) {
	type payload struct {
		Name string `json:"name" validate:"required"`
	}

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	var p payload
	err := decodeRequestBodyJSON(req, &p)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// ---------- JSONHandler ----------

func TestJSONHandler_GET_NoBody(t *testing.T) {
	type req struct{}
	type resp struct {
		Message string `json:"message"`
	}

	handler := JSONHandler(http.StatusOK, func(r *http.Request, body req) (resp, error) {
		return resp{Message: "hello"}, nil
	})

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var apiResp apiResponse
	json.NewDecoder(rec.Body).Decode(&apiResp)
	if len(apiResp.Errors) != 0 {
		t.Errorf("unexpected errors: %+v", apiResp.Errors)
	}
}

func TestJSONHandler_POST_Valid(t *testing.T) {
	type req struct {
		Name string `json:"name"`
	}
	type resp struct {
		Greeting string `json:"greeting"`
	}

	handler := JSONHandler(http.StatusCreated, func(r *http.Request, body req) (resp, error) {
		return resp{Greeting: "hi " + body.Name}, nil
	})

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"world"}`))
	handler.ServeHTTP(rec, r)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestJSONHandler_POST_InvalidBody(t *testing.T) {
	type req struct {
		Name string `json:"name"`
	}

	handler := JSONHandler(http.StatusOK, func(r *http.Request, body req) (req, error) {
		return body, nil
	})

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{invalid}"))
	handler.ServeHTTP(rec, r)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestJSONHandler_POST_ValidationError(t *testing.T) {
	type req struct {
		Name string `json:"name" validate:"required"`
	}

	handler := JSONHandler(http.StatusOK, func(r *http.Request, body req) (req, error) {
		return body, nil
	})

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	handler.ServeHTTP(rec, r)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestJSONHandler_HTTPError(t *testing.T) {
	type req struct{}

	handler := JSONHandler(http.StatusOK, func(r *http.Request, body req) (req, error) {
		return req{}, HTTPError{Status: http.StatusNotFound, Message: "not found"}
	})

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var apiResp apiResponse
	json.NewDecoder(rec.Body).Decode(&apiResp)
	if len(apiResp.Errors) != 1 || apiResp.Errors[0].Message != "not found" {
		t.Errorf("errors = %+v", apiResp.Errors)
	}
}

func TestJSONHandler_InternalError(t *testing.T) {
	type req struct{}

	handler := JSONHandler(http.StatusOK, func(r *http.Request, body req) (req, error) {
		return req{}, errors.New("boom")
	})

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, r)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var apiResp apiResponse
	json.NewDecoder(rec.Body).Decode(&apiResp)
	if len(apiResp.Errors) != 1 || apiResp.Errors[0].Message != "internal server error" {
		t.Errorf("errors = %+v", apiResp.Errors)
	}
}
