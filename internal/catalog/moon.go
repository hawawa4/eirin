package catalog

import (
	"math"
	"time"
)

// MoonIllumination returns the fraction of the moon's disk that is illuminated
// (0.0 = new moon, 1.0 = full moon) for a given UTC time.
// Uses the algorithm from "Astronomical Algorithms" (Meeus, ch.48).
func MoonIllumination(t time.Time) float64 {
	jd := julianDay(t)

	// Time in Julian centuries from J2000.0
	T := (jd - 2451545.0) / 36525.0

	// Sun's mean anomaly (degrees)
	M := 357.5291092 + 35999.0502909*T - 0.0001536*T*T + T*T*T/24490000.0
	M = math.Mod(M, 360)
	if M < 0 {
		M += 360
	}

	// Moon's mean anomaly (degrees)
	Mprime := 134.9633964 + 477198.8675055*T + 0.0087414*T*T + T*T*T/69699.0 - T*T*T*T/14712000.0
	Mprime = math.Mod(Mprime, 360)
	if Mprime < 0 {
		Mprime += 360
	}

	// Moon's argument of latitude (degrees)
	F := 93.2720950 + 483202.0175233*T - 0.0036539*T*T - T*T*T/3526000.0 + T*T*T*T/863310000.0
	F = math.Mod(F, 360)
	if F < 0 {
		F += 360
	}

	// Moon's mean elongation from the Sun (degrees)
	D := 297.8501921 + 445267.1114034*T - 0.0018819*T*T + T*T*T/545868.0 - T*T*T*T/113065000.0
	D = math.Mod(D, 360)
	if D < 0 {
		D += 360
	}

	// Convert to radians
	Mrad := deg2rad(M)
	Mprad := deg2rad(Mprime)
	Drad := deg2rad(D)
	Frad := deg2rad(F)

	// Geocentric elongation of the Moon from the Sun (approximate)
	// ψ ≈ D + perturbations
	psi := 180 - D -
		6.289*math.Sin(Mprad) +
		2.100*math.Sin(Mrad) -
		1.274*math.Sin(2*Drad-Mprad) -
		0.658*math.Sin(2*Drad) -
		0.214*math.Sin(2*Mprad) -
		0.110*math.Sin(Drad) +
		0.040*math.Sin(2*Frad)
	_ = psi

	// Phase angle i (degrees): angle Sun–Moon–Earth
	// For illumination fraction k = (1 + cos(i)) / 2
	// A simpler but accurate version: use synodic phase from elongation D
	phaseAngle := 180 - D -
		6.289*math.Sin(Mprad) +
		2.100*math.Sin(Mrad) -
		1.274*math.Sin(2*Drad-Mprad) -
		0.658*math.Sin(2*Drad) -
		0.214*math.Sin(2*Mprad) -
		0.110*math.Sin(Drad)

	phaseRad := deg2rad(phaseAngle)
	k := (1 + math.Cos(phaseRad)) / 2.0
	if k < 0 {
		k = 0
	}
	if k > 1 {
		k = 1
	}
	return k
}

func julianDay(t time.Time) float64 {
	t = t.UTC()
	y := float64(t.Year())
	m := float64(t.Month())
	d := float64(t.Day()) + (float64(t.Hour())+float64(t.Minute())/60.0+float64(t.Second())/3600.0)/24.0

	if m <= 2 {
		y--
		m += 12
	}
	A := math.Floor(y / 100)
	B := 2 - A + math.Floor(A/4)
	return math.Floor(365.25*(y+4716)) + math.Floor(30.6001*(m+1)) + d + B - 1524.5
}

func deg2rad(d float64) float64 { return d * math.Pi / 180 }
