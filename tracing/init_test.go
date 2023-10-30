package tracing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	tt := []struct {
		name        string
		serviceName string

		expectedErrorStr string
	}{
		{
			name:        "success",
			serviceName: "test",
		},
		{
			name:        "failed to initialize jaeger tracer",
			serviceName: "",

			expectedErrorStr: "cannot initialize jaeger tracer",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tracer, closer, err := InitTracer("")

			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			require.NotNil(t, tracer)
			require.NotNil(t, closer)
		})
	}
}
