package timeprovider

import "time"

type TimeProvider struct {
}

func New() *TimeProvider {
	return &TimeProvider{}
}

func (tp *TimeProvider) Now() time.Time {
	return time.Now()
}

func (tp *TimeProvider) Add(d time.Duration) time.Time {
	return tp.Now().Add(d)
}
