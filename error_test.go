package vivid_test

import (
	"errors"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
)

type testWrappedErr struct{}

func (testWrappedErr) Error() string {
	return "wrapped"
}

func findUnusedErrorCode(t *testing.T) int32 {
	t.Helper()

	for code := int32(2000000000); code > 0; code++ {
		if vivid.QueryError(code) == nil {
			return code
		}
	}

	t.Fatal("no unused error code available")
	return 0
}

func TestRegisterAndQueryError(t *testing.T) {
	code := findUnusedErrorCode(t)
	registered := vivid.RegisterError(code, "test error")

	assert.NotNil(t, registered)
	assert.Equal(t, code, registered.GetCode())
	assert.Equal(t, "test error", registered.GetMessage())

	assert.Equal(t, registered, vivid.QueryError(code))

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on duplicate error code")
		}
	}()
	_ = vivid.RegisterError(code, "duplicate")
}

func TestErrorWithAndWithMessage(t *testing.T) {
	base := vivid.ErrorNotFound

	assert.Equal(t, base, base.With(nil))
	assert.Equal(t, base, base.WithMessage(""))

	wrapped := base.With(errors.New("detail"))
	assert.Equal(t, base.GetCode(), wrapped.GetCode())
	assert.NotEqual(t, base.GetMessage(), wrapped.GetMessage())

	withMsg := base.WithMessage("detail")
	assert.Equal(t, base.GetCode(), withMsg.GetCode())
	assert.NotEqual(t, base.GetMessage(), withMsg.GetMessage())
}

func TestErrorIsAsUnwrap(t *testing.T) {
	base := vivid.ErrorIllegalArgument
	sentinel := testWrappedErr{}

	wrapped := base.With(sentinel)
	assert.True(t, errors.Is(wrapped, base))
	assert.True(t, errors.Is(wrapped, sentinel))

	var got *vivid.Error
	assert.True(t, errors.As(wrapped, &got))
	assert.Equal(t, got, wrapped)

	var gotWrapped testWrappedErr
	assert.True(t, errors.As(wrapped, &gotWrapped))
	assert.Equal(t, errors.Unwrap(wrapped), sentinel)

	withMsg := base.WithMessage("detail")
	assert.True(t, errors.Is(withMsg, base))
	assert.Equal(t, errors.Unwrap(withMsg), base)
}
