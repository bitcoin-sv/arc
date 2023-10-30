package tracing

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	tt := []struct {
		name        string
		serviceName string
		setEnv      bool

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
		{
			name:   "config error",
			setEnv: true,

			expectedErrorStr: "cannot parse jaeger env vars",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnv {
				os.Setenv("JAEGER_RPC_METRICS", "not a boolean")
			}

			tracer, closer, err := InitTracer(tc.serviceName)

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}

			require.NotNil(t, tracer)
			require.NotNil(t, closer)
		})
	}
}
