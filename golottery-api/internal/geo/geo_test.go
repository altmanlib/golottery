package geo

import (
	"math"
	"testing"
)

func TestDistance(t *testing.T) {
	// Tiananmen to the Forbidden City's north gate, about 1.7 km apart.
	d := DistanceM(39.908823, 116.397470, 39.924091, 116.403414)
	if d < 1650 || d > 1800 {
		t.Fatalf("distance = %.0f m", d)
	}
	if DistanceM(31.23, 121.47, 31.23, 121.47) != 0 {
		t.Fatal("distance to itself is not zero")
	}
	// 0.001 degree of latitude is about 111 m everywhere.
	if d := DistanceM(30, 120, 30.001, 120); math.Abs(d-111.2) > 1 {
		t.Fatalf("0.001° latitude = %.1f m", d)
	}
}

func TestWGS84ToGCJ02(t *testing.T) {
	// A published reference pair in Beijing; GCJ-02 shifts a few hundred meters.
	lat, lng := WGS84ToGCJ02(39.907500, 116.391200)
	if math.Abs(lat-39.908903) > 0.00005 || math.Abs(lng-116.397450) > 0.00005 {
		t.Fatalf("converted = %.6f, %.6f", lat, lng)
	}
	if shift := DistanceM(39.9075, 116.3912, lat, lng); shift < 300 || shift > 700 {
		t.Fatalf("shift = %.0f m, want a few hundred meters", shift)
	}
	// Outside China nothing moves.
	if lat, lng := WGS84ToGCJ02(51.5007, -0.1246); lat != 51.5007 || lng != -0.1246 {
		t.Fatalf("London moved to %f, %f", lat, lng)
	}
}

func TestValid(t *testing.T) {
	if !Valid(0, 0) || Valid(91, 0) || Valid(0, 181) || Valid(math.NaN(), 0) {
		t.Fatal("Valid misjudged a coordinate")
	}
}
