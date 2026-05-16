package app

import "math"

// Annotation is a labeled point projected onto the image plane.
type Annotation struct {
	X     float64 `json:"x"`     // pixel column from top-left
	Y     float64 `json:"y"`     // pixel row from top-left
	Label string  `json:"label"`
	Type  string  `json:"type"` // "star" or "dso"
	Mag   float64 `json:"mag"`  // visual magnitude (0 for DSOs)
}

type catalogStar struct {
	RA, Dec float64
	Name    string
	Mag     float64
	Type    string
}

// brightStars is a hand-picked subset of the Yale Bright Star Catalog (mag ≤ 2.5)
// plus a handful of prominent fainter stars useful for identification.
// RA in decimal degrees, Dec in decimal degrees.
var brightStars = []catalogStar{
	{79.172, 45.998, "Capella", 0.08, "star"},
	{88.793, 7.407, "Betelgeuse", 0.50, "star"},
	{79.172, -8.202, "Rigel", 0.13, "star"},
	{101.287, -16.716, "Sirius", -1.46, "star"},
	{95.988, -52.696, "Canopus", -0.72, "star"},
	{114.826, 5.225, "Procyon", 0.34, "star"},
	{213.915, 19.182, "Arcturus", -0.05, "star"},
	{279.235, 38.784, "Vega", 0.03, "star"},
	{297.695, 8.868, "Altair", 0.76, "star"},
	{310.358, 45.280, "Deneb", 1.25, "star"},
	{191.930, -59.689, "Acrux", 0.76, "star"},
	{186.650, -63.099, "Mimosa", 1.25, "star"},
	{152.093, 11.967, "Regulus", 1.35, "star"},
	{177.265, 14.572, "Denebola", 2.14, "star"},
	{68.980, 16.509, "Aldebaran", 0.85, "star"},
	{84.411, -1.202, "Bellatrix", 1.64, "star"},
	{81.570, 28.608, "Alnath", 1.65, "star"},
	{85.190, -1.943, "Alnilam", 1.69, "star"},
	{83.858, -5.909, "Alnitak", 1.74, "star"},
	{86.939, 1.724, "Mintaka", 2.23, "star"},
	{116.329, 28.026, "Pollux", 1.14, "star"},
	{113.650, 31.888, "Castor", 1.58, "star"},
	{24.428, -57.237, "Achernar", 0.46, "star"},
	{210.956, -60.374, "Hadar", 0.61, "star"},
	{219.899, -60.834, "Rigil Kent.", 0.01, "star"},
	{263.054, -37.103, "Shaula", 1.62, "star"},
	{247.352, -26.432, "Antares", 1.06, "star"},
	{344.413, -29.622, "Fomalhaut", 1.16, "star"},
	{337.821, -46.961, "Alnair", 1.74, "star"},
	{346.190, -15.821, "Sadalsuud", 2.91, "star"},
	{10.897, 56.537, "Schedar", 2.24, "star"},
	{9.243, 59.150, "Caph", 2.27, "star"},
	{3.309, 15.183, "Alpheratz", 2.06, "star"},
	{351.303, -46.018, "Peacock", 1.94, "star"},
	{252.166, -69.028, "Atria", 1.91, "star"},
	{164.012, -49.421, "Avior", 1.86, "star"},
	{138.300, -69.717, "Miaplacidus", 1.67, "star"},
	{120.896, -40.003, "Naos", 2.25, "star"},
	{155.582, 19.842, "Alphard", 1.98, "star"},
	{141.897, -8.659, "Gomeisa", 2.90, "star"},
	{222.676, -16.042, "Zubenelg.", 2.61, "star"},
	{232.173, -40.648, "Menkent", 2.06, "star"},
	{258.662, 14.390, "Rasalhague", 2.08, "star"},
	{269.152, 51.489, "Eltanin", 2.24, "star"},
	{286.352, -21.106, "Nunki", 2.05, "star"},
	{283.816, -26.296, "Ascella", 2.60, "star"},
	{311.553, -40.616, "Al Na'ir", 2.17, "star"},
	{322.164, -16.662, "Sadalmelik", 2.95, "star"},
}

// commonDSOs contains well-known deep-sky objects with approximate center coordinates.
var commonDSOs = []catalogStar{
	{83.822, -5.391, "M42 Orion Neb.", 4.0, "dso"},
	{83.840, -5.275, "M43", 9.0, "dso"},
	{101.289, -2.099, "M50", 5.9, "dso"},
	{80.894, 41.270, "M1 Crab", 8.4, "dso"},
	{148.888, 69.065, "M82", 8.4, "dso"},
	{148.969, 69.679, "M81", 6.9, "dso"},
	{186.266, 47.303, "M51 Whirlpool", 8.4, "dso"},
	{202.469, 47.195, "M63", 8.6, "dso"},
	{185.729, 15.822, "M64", 8.5, "dso"},
	{201.365, -43.019, "Cen A", 6.8, "dso"},
	{187.706, 12.391, "M87 Virgo A", 8.6, "dso"},
	{189.998, 11.553, "M84", 9.1, "dso"},
	{190.916, 12.887, "M86", 8.9, "dso"},
	{284.275, -2.283, "M20 Trifid", 6.3, "dso"},
	{271.119, -23.028, "M8 Lagoon", 5.8, "dso"},
	{272.556, -19.017, "M17 Omega", 6.0, "dso"},
	{274.700, -13.792, "M16 Eagle", 6.4, "dso"},
	{290.365, 18.534, "M27 Dumbbell", 7.4, "dso"},
	{299.017, 22.721, "M71", 6.1, "dso"},
	{305.557, 40.917, "M29", 6.6, "dso"},
	{316.757, 30.227, "NGC 7331", 9.5, "dso"},
	{322.524, 12.187, "M15", 6.2, "dso"},
	{333.612, 34.398, "M31 Andromeda", 3.4, "dso"},
	{335.692, 41.269, "M32", 8.7, "dso"},
	{335.993, 41.686, "M110", 8.0, "dso"},
	{10.685, 41.269, "M32 sat", 8.7, "dso"},
	{23.462, 30.660, "M33 Triangulum", 5.7, "dso"},
	{40.670, 41.269, "NGC 891", 9.9, "dso"},
	{52.675, 34.316, "NGC 1499 Calif.", 5.0, "dso"},
	{69.066, 24.105, "M45 Pleiades", 1.6, "dso"},
	{66.896, 15.952, "M1", 8.4, "dso"},
	{94.033, 22.014, "IC 443", 12.0, "dso"},
	{162.020, 57.015, "M97 Owl Neb.", 9.9, "dso"},
	{218.475, -13.793, "M68", 7.6, "dso"},
	{250.423, 36.460, "M13 Hercules", 5.8, "dso"},
	{254.688, 43.136, "M92", 6.4, "dso"},
	{262.017, 18.546, "M12", 6.6, "dso"},
	{263.548, 26.103, "M10", 6.6, "dso"},
	{323.361, -0.815, "M2", 6.5, "dso"},
	{325.089, -0.823, "M72", 9.3, "dso"},
	{330.015, -0.823, "M73", 9.0, "dso"},
	{351.202, 57.134, "M52", 6.9, "dso"},
	{357.034, 56.862, "NGC 7789", 6.7, "dso"},
	{10.625, 48.333, "NGC 224", 3.4, "dso"},
	{150.980, 2.640, "M95", 9.7, "dso"},
	{161.690, 13.096, "M96", 9.2, "dso"},
	{168.634, 12.581, "M105", 9.3, "dso"},
	{13.158, -25.262, "NGC 253 Sculptor", 7.1, "dso"},
	{23.924, -20.833, "NGC 300", 8.7, "dso"},
}

// GetAnnotations returns catalog objects visible within the frame defined by the
// plate-solve result (raCenter/decCenter in degrees, pixelScale in arcsec/pixel,
// rotation in degrees CCW from North, image dimensions in pixels).
// Returns objects whose projected pixel position falls inside the image.
func (a *App) GetAnnotations(raCenter, decCenter, pixelScale, rotation float64, width, height int) []Annotation {
	if pixelScale <= 0 || width <= 0 || height <= 0 {
		return nil
	}

	all := make([]catalogStar, 0, len(brightStars)+len(commonDSOs))
	all = append(all, brightStars...)
	all = append(all, commonDSOs...)

	ra0 := deg2rad(raCenter)
	dec0 := deg2rad(decCenter)
	rotRad := deg2rad(rotation)
	scale := pixelScale / 3600.0 // arcsec/px → deg/px
	scaleDeg := deg2rad(scale)    // rad/px

	cx := float64(width) / 2
	cy := float64(height) / 2

	var out []Annotation
	for _, s := range all {
		ra := deg2rad(s.RA)
		dec := deg2rad(s.Dec)

		// Gnomonic (tangential) projection
		dRA := ra - ra0
		denom := math.Sin(dec0)*math.Sin(dec) + math.Cos(dec0)*math.Cos(dec)*math.Cos(dRA)
		if denom <= 0 {
			continue
		}
		xi := math.Cos(dec) * math.Sin(dRA) / denom
		eta := (math.Cos(dec0)*math.Sin(dec) - math.Sin(dec0)*math.Cos(dec)*math.Cos(dRA)) / denom

		// xi, eta are in radians; convert to pixels
		xPx := xi / scaleDeg
		yPx := -eta / scaleDeg // flip: eta increases north but y increases down

		// Apply field rotation
		cosR := math.Cos(rotRad)
		sinR := math.Sin(rotRad)
		xRot := xPx*cosR - yPx*sinR
		yRot := xPx*sinR + yPx*cosR

		// Translate to image coordinates
		px := cx + xRot
		py := cy + yRot

		margin := 20.0
		if px < -margin || px > float64(width)+margin || py < -margin || py > float64(height)+margin {
			continue
		}

		out = append(out, Annotation{
			X:     math.Round(px*10) / 10,
			Y:     math.Round(py*10) / 10,
			Label: s.Name,
			Type:  s.Type,
			Mag:   s.Mag,
		})
	}
	return out
}
