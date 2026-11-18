// Package geo measures distance on the earth's surface.
package geo

import "math"

const earthRadiusM = 6371000.0

// DistanceM returns the great-circle distance between two points in metres.
func DistanceM(lat1, lng1, lat2, lng2 float64) float64 {
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLng := rad(lat2-lat1), rad(lng2-lng1)
	a := math.Pow(math.Sin(dLat/2), 2) + math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Pow(math.Sin(dLng/2), 2)
	return 2 * earthRadiusM * math.Asin(math.Min(1, math.Sqrt(a)))
}
