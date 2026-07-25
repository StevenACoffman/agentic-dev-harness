// Package device abstracts on-device (adb) validation (SPEC §4). The real
// adapter shells out to adb against hardware; Mock returns a fixed report so the
// loop and its tests run without a device.
package device

import "context"

// Report is the outcome of an on-device validation run.
type Report struct {
	OK     bool
	Detail string
}

// Mock is a deterministic on-device validator for tests and the mock backend.
type Mock struct {
	Healthy bool
}

// Validate returns a fixed report reflecting m.Healthy; it never touches adb.
func (m Mock) Validate(_ context.Context) (Report, error) {
	if m.Healthy {
		return Report{OK: true, Detail: "mock device healthy"}, nil
	}
	return Report{OK: false, Detail: "mock device unhealthy"}, nil
}
