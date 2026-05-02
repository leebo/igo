package errors

import (
	stderrors "errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStructuredErrorBuilderAndErrorString(t *testing.T) {
	err := NewStructuredError(CodeValidation, "invalid").
		WithField("email").
		WithRule("email").
		WithValue("bad").
		WithFilePath("handler.go").
		WithLine(42).
		WithSuggestions("fix it").
		WithMetadata("request_id", "abc")

	assert.Equal(t, "email: invalid (email)", err.Error())
	assert.Equal(t, CodeValidation, err.Code)
	assert.Equal(t, "bad", err.Value)
	assert.Equal(t, "handler.go", err.FilePath)
	assert.Equal(t, 42, err.Line)
	assert.Equal(t, []string{"fix it"}, err.Suggestions)
	assert.Equal(t, "abc", err.Metadata["request_id"])
}

func TestStructuredErrorWrapAndCallChain(t *testing.T) {
	root := NewStructuredError(CodeBadRequest, "root").AddCallFrame()
	wrapped := NewStructuredError(CodeBadRequest, "outer").Wrap(root, "wrapped")

	require.NotNil(t, wrapped.RootCause)
	assert.Equal(t, root, wrapped.RootCause)
	assert.Equal(t, "wrapped", wrapped.Message)
	assert.ErrorIs(t, wrapped, root)
	assert.NotEmpty(t, wrapped.CallChain)
	assert.NotEmpty(t, wrapped.GetCallChain())

	plain := NewStructuredError(CodeInternalError, "outer").Wrap(stderrors.New("disk"), "wrapped")
	require.NotNil(t, plain.RootCause)
	assert.Equal(t, "disk", plain.RootCause.Message)
}

func TestStructuredErrorSuggestionsAndStatusCodes(t *testing.T) {
	tests := map[string][]string{
		"required": {"确保字段有值", "检查字段是否正确绑定"},
		"email":    {"检查邮箱格式是否为 user@example.com", "确保没有多余空格"},
		"unknown":  {"检查字段格式是否正确", "查看 validate tag 定义"},
	}

	for rule, want := range tests {
		t.Run(rule, func(t *testing.T) {
			got := NewValidationError("field", rule, "invalid").WithSuggestionsForValidation()
			assert.Equal(t, want, got.Suggestions)
		})
	}

	assert.Equal(t, CodeBadRequest, ErrorCodeFromStatus(http.StatusBadRequest))
	assert.Equal(t, CodeUnauthorized, ErrorCodeFromStatus(http.StatusUnauthorized))
	assert.Equal(t, CodeForbidden, ErrorCodeFromStatus(http.StatusForbidden))
	assert.Equal(t, CodeNotFound, ErrorCodeFromStatus(http.StatusNotFound))
	assert.Equal(t, CodeValidation, ErrorCodeFromStatus(http.StatusUnprocessableEntity))
	assert.Equal(t, CodeInternalError, ErrorCodeFromStatus(http.StatusTeapot))
}

func TestErrorResponse(t *testing.T) {
	root := NewStructuredError(CodeBadRequest, "root").WithField("name")
	err := NewStructuredError(CodeBadRequest, "bad").
		WithField("name").
		WithFilePath("handler.go").
		WithLine(12).
		WithSuggestions("rename").
		WithMetadata("trace", "abc").
		WithRootCause(root).
		WithCallChain([]CallFrame{{FunctionName: "handler"}})

	resp := NewErrorResponse(err)

	assert.Equal(t, CodeBadRequest, resp.Error.Code)
	assert.Equal(t, "bad", resp.Error.Message)
	assert.Equal(t, "name", resp.Error.Field)
	assert.Equal(t, []string{"rename"}, resp.Error.Suggestions)
	require.NotNil(t, resp.Error.Context)
	assert.Equal(t, "handler.go", resp.Error.Context.FilePath)
	assert.Equal(t, "abc", resp.Error.Metadata["trace"])
	require.NotNil(t, resp.Error.RootCause)
	assert.Equal(t, "root", resp.Error.RootCause.Message)
	assert.Equal(t, "[BAD_REQUEST] bad", resp.String())

	unknown := NewErrorResponse(nil)
	assert.Equal(t, CodeInternalError, unknown.Error.Code)
}

func TestValidationErrorResponsesAndHints(t *testing.T) {
	empty := NewErrorResponseFromValidationErrors(ValidationErrors{})
	assert.Equal(t, CodeValidation, empty.Error.Code)
	assert.Equal(t, "validation failed", empty.Error.Message)

	single := NewErrorResponseFromValidationErrors(ValidationErrors{Errors: []StructuredError{
		*NewValidationError("email", "email", "bad email").WithSuggestions("fix email"),
	}})
	assert.Equal(t, "email", single.Error.Field)
	assert.Equal(t, []string{"fix email"}, single.Error.Suggestions)

	multiple := NewErrorResponseFromValidationErrors(ValidationErrors{Errors: []StructuredError{
		*NewValidationError("email", "email", "bad email"),
		*NewValidationError("name", "required", "missing"),
	}})
	assert.Equal(t, "multiple validation errors", multiple.Error.Message)
	require.Len(t, multiple.Error.Details, 2)
	assert.Equal(t, "email", multiple.Error.Details[0].Field)

	resp := SimpleErrorResponse(CodeForbidden, "forbidden")
	resp.WithSuggestions("login").WithContext(&ErrorContext{FilePath: "auth.go", Line: 1})
	assert.Equal(t, []string{"login"}, resp.Error.Suggestions)
	require.NotNil(t, resp.Error.Context)
	assert.Equal(t, "auth.go", resp.Error.Context.FilePath)

	assert.NotEmpty(t, GetHintForRule("required"))
	assert.Equal(t, []string{"检查字段格式是否正确"}, GetHintForRule("missing"))
}

func TestListErrorCodes(t *testing.T) {
	codes := ListErrorCodes()
	require.NotEmpty(t, codes)

	got := map[string]int{}
	for _, code := range codes {
		got[code.Code] = code.StatusCode
		assert.NotEmpty(t, code.HelperPlain)
		assert.NotEmpty(t, code.Description)
	}

	assert.Equal(t, http.StatusBadRequest, got[CodeBadRequest])
	assert.Equal(t, http.StatusInternalServerError, got[CodeInternalError])
	assert.Equal(t, http.StatusUnprocessableEntity, got[CodeValidation])
}
