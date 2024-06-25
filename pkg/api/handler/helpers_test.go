package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/bitcoin-sv/arc/pkg/blocktx"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestCheckSwagger(t *testing.T) {
	tt := []struct {
		name string
		path string
	}{
		{
			name: "valid request",
			path: "/v1/policy",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(""))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			swagger := CheckSwagger(e)
			assert.NotNil(t, swagger)
		})
	}
}

func TestConvertMerkleRootsRequest(t *testing.T) {
	testCases := []struct {
		name            string
		request         []beef.MerkleRootVerificationRequest
		expectedRequest []blocktx.MerkleRootVerificationRequest
	}{
		{
			name: "",
			request: []beef.MerkleRootVerificationRequest{
				{
					MerkleRoot:  "1234",
					BlockHeight: 818000,
				},
				{
					MerkleRoot:  "5678",
					BlockHeight: 818001,
				},
			},
			expectedRequest: []blocktx.MerkleRootVerificationRequest{
				{
					MerkleRoot:  "1234",
					BlockHeight: 818000,
				},
				{
					MerkleRoot:  "5678",
					BlockHeight: 818001,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := convertMerkleRootsRequest(tc.request)
			assert.Equal(t, tc.expectedRequest, request)
		})
	}
}
