package callbacker

import "time"

type quarantinePolicy struct {
	baseDuration        time.Duration
	permQuarantineAfter time.Duration
	now                 func() time.Time
}

// arbitrarily chosen date in a distant future
var infinity = time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)

// Until calculates the time until which the quarantine should last based on a reference time.
// It compares the current time with the given referenceTime and adjusts the duration of quarantine as follows:
//
// 1. If the time since referenceTime is less than or equal to baseDuration:
//   - The quarantine ends after the baseDuration has passed from the current time.
//   - Returns the current time plus baseDuration.
//
// 2. If the time since referenceTime is greater than baseDuration but less than permQuarantineAfter:
//   - The quarantine duration is extended to double the time that has passed since the referenceTime.
//   - Returns the current time plus twice the elapsed time.
//
// 3. If the time since referenceTime exceeds permQuarantineAfter:
//   - The quarantine is considered "permanent", meaning no further action is needed.
//   - Returns a predefined "infinity" date (January 1, 2999).
//
// This function dynamically adjusts the quarantine period based on how long it has been since
// the reference time, with the possibility of an extended period or abandonment after a certain threshold.
func (p *quarantinePolicy) Until(referenceTime time.Time) time.Time {
	duration := p.baseDuration

	since := p.now().Sub(referenceTime)
	if since > p.baseDuration {
		if since > p.permQuarantineAfter {
			return infinity
		}

		duration = since * 2
	}

	return p.now().Add(duration)
}
