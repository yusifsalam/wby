package weather

import (
	"errors"
	"time"
)

// ErrPrecipitationDisabled is returned when a precipitation grid source is
// unconfigured or holds no frame for the requested time.
var ErrPrecipitationDisabled = errors.New("precipitation overlay disabled")

// PrecipitationOverlayRequest extends MapOverlayRequest with a target time.
type PrecipitationOverlayRequest struct {
	MapOverlayRequest
	Time time.Time
}
