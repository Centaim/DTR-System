package geo

import (
	"math"
	"testing"
)

func TestDistance(t *testing.T) {
	if d := DistanceM(14.5995, 120.9842, 14.5995, 120.9842); d != 0 {
		t.Errorf("same point: %v", d)
	}
	// Manila to Cebu City is about 571 km.
	if d := DistanceM(14.5995, 120.9842, 10.3157, 123.8854); math.Abs(d-571000) > 5000 {
		t.Errorf("Manila to Cebu: %v", d)
	}
	// 0.001 degree of latitude is about 111 m.
	if d := DistanceM(14.0, 121.0, 14.001, 121.0); math.Abs(d-111) > 1 {
		t.Errorf("short distance: %v", d)
	}
}
