package callbacker

import "time"

type quarantinePolicy struct {
	baseDuration time.Duration
	abandonAfter time.Duration
	now          func() time.Time
}

var infinity = time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)

func (p *quarantinePolicy) Until(referenceTime time.Time) time.Time {
	duration := p.baseDuration

	since := p.now().Sub(referenceTime)
	if since > p.baseDuration {
		if since > p.abandonAfter {
			return infinity
		}

		duration = since * 2
	}

	return p.now().Add(duration)
}
