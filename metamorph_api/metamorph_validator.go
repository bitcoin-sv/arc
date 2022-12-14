package metamorph_api

import (
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// Validate validates the EncryptRequest and returns error if it finds any issue
func (v *HealthResponse) Validate() error {
	if v.Timestamp == nil {
		responseStatus := status.New(codes.InvalidArgument, "Timestamp must be populated")
		return responseStatus.Err()
	}

	return nil
}
