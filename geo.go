package main

import (
	"math"
	"github.com/mmcloughlin/geohash"
)

func haversineDistance(lat1, long1, lat2, long2 float64) float64 {
	const earthRadiusKm = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLong := (long2 - long1) * math.Pi / 180
	lat1R := lat1 * math.Pi / 180
	lat2R := lat2 * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1R)*math.Cos(lat2R)*math.Sin(dLong/2)*math.Sin(dLong/2)
	return earthRadiusKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func encodeGeohash(lat, lng float64, precision int) string {
	return geohash.EncodeWithPrecision(lat, lng, uint(precision))
}

func geohashNeighbors(hash string) []string {
	return geohash.Neighbors(hash)
}
