package util

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValueFromCtx(t *testing.T) {
	type TestStruct struct {
		Field string
	}

	tests := []struct {
		name        string
		setupCtx    func() context.Context
		key         CtxKey
		wantValue   interface{}
		wantErrCode int64
	}{
		{
			name: "string value - success",
			setupCtx: func() context.Context {
				ctx := context.Background()
				return ValueToCtx(ctx, "string-key", "test-value")
			},
			key:       "string-key",
			wantValue: "test-value",
		},
		{
			name: "int value - success",
			setupCtx: func() context.Context {
				ctx := context.Background()
				return ValueToCtx(ctx, "int-key", 42)
			},
			key:       "int-key",
			wantValue: 42,
		},
		{
			name: "struct value - success",
			setupCtx: func() context.Context {
				ctx := context.Background()
				return ValueToCtx(ctx, "struct-key", TestStruct{Field: "test"})
			},
			key:       "struct-key",
			wantValue: TestStruct{Field: "test"},
		},
		{
			name: "pointer value - success",
			setupCtx: func() context.Context {
				ctx := context.Background()
				val := &TestStruct{Field: "test"}
				return ValueToCtx(ctx, "ptr-key", val)
			},
			key:       "ptr-key",
			wantValue: &TestStruct{Field: "test"},
		},
		{
			name: "nil value - error",
			setupCtx: func() context.Context {
				return context.Background()
			},
			key:         "missing-key",
			wantErrCode: ErrCodeValueNotFoundInContext,
		},
		{
			name: "wrong type - error",
			setupCtx: func() context.Context {
				ctx := context.Background()
				return ValueToCtx(ctx, "wrong-type", "string-value")
			},
			key:         "wrong-type",
			wantErrCode: ErrCodeInvalidValueInContext,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()

			var utilErr *UtilError
			switch expected := tt.wantValue.(type) {
			case string:
				got, err := ValueFromCtx[string](ctx, tt.key)
				if tt.wantErrCode != 0 {
					assert.Error(t, err)
					utilErr = err.(*UtilError)
					assert.Equal(t, tt.wantErrCode, utilErr.GetCode())
				} else {
					assert.NoError(t, err)
					assert.Equal(t, expected, got)
				}
			case int:
				got, err := ValueFromCtx[int](ctx, tt.key)
				if tt.wantErrCode != 0 {
					assert.Error(t, err)
					utilErr = err.(*UtilError)
					assert.Equal(t, tt.wantErrCode, utilErr.GetCode())
				} else {
					assert.NoError(t, err)
					assert.Equal(t, expected, got)
				}
			case TestStruct:
				got, err := ValueFromCtx[TestStruct](ctx, tt.key)
				if tt.wantErrCode != 0 {
					assert.Error(t, err)
					utilErr = err.(*UtilError)
					assert.Equal(t, tt.wantErrCode, utilErr.GetCode())
				} else {
					assert.NoError(t, err)
					assert.Equal(t, expected, got)
				}
			case *TestStruct:
				got, err := ValueFromCtx[*TestStruct](ctx, tt.key)
				if tt.wantErrCode != 0 {
					assert.Error(t, err)
					utilErr = err.(*UtilError)
					assert.Equal(t, tt.wantErrCode, utilErr.GetCode())
				} else {
					assert.NoError(t, err)
					assert.Equal(t, expected, got)
				}
			}
		})
	}
}
