package gallery

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Processed images are usually exports (PNG/JPEG/TIFF) with no FITS header, so
// most of what is known about them comes from the file name: Seestar names look
// like "Stacked_30_M 42_10.0s_IRCUT_20240101-221530.jpg". This mirrors the
// desktop's frontend/src/lib/slideshow/frameInfo.ts, but runs on the server so
// the file name itself never reaches visitors. (Go's regexp has no lookaround,
// so boundaries are matched as "start or a non-word character".)
var (
	reScope    = regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])(S50|S30)(?:[\s_-]?(Pro))?(?:$|[^A-Za-z0-9])`)
	reCombined = regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])(\d+)\s?[x×]\s?(\d+(?:\.\d+)?)\s?s(?:$|[^A-Za-z])`)
	reCount    = regexp.MustCompile(`(?i)^(?:stacked|stack|sum)[_\s-](\d+)(?:$|[^\d.])`)
	reExposure = regexp.MustCompile(`(?:^|[^A-Za-z0-9.])(\d+(?:\.\d+)?)\s?s(?:$|[^A-Za-z])`)
	reFilter   = regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])(IR-?CUT|LP|DUO-?BAND)(?:$|[^A-Za-z0-9])`)
	reDate     = regexp.MustCompile(`(?:^|\D)((?:19|20)\d{2})[-_]?(0[1-9]|1[0-2])[-_]?(0[1-9]|[12]\d|3[01])(?:$|\D)`)
	reDateObs  = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})`)
	reSerial   = regexp.MustCompile(`(?i)^(.+)_[0-9a-f]{4,}$`)
)

// NameMeta is what a file name reveals; every field is optional.
type NameMeta struct {
	Telescope string
	// Subs is the number of stacked subs, SubExposure one sub's seconds.
	Subs        int
	SubExposure float64
	Filter      string
	// Date is the local calendar day, "2006-01-02".
	Date string
}

// ParseName reads whatever fileName says about the capture.
func ParseName(fileName string) NameMeta {
	stem := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	var m NameMeta

	if s := reScope.FindStringSubmatch(stem); s != nil {
		m.Telescope = strings.ToUpper(s[1])
		if s[2] != "" {
			m.Telescope += " Pro"
		}
	}

	if s := reCombined.FindStringSubmatch(stem); s != nil {
		m.Subs, _ = strconv.Atoi(s[1])
		m.SubExposure, _ = strconv.ParseFloat(s[2], 64)
	} else {
		if s := reCount.FindStringSubmatch(stem); s != nil {
			m.Subs, _ = strconv.Atoi(s[1])
		}
		if s := reExposure.FindStringSubmatch(stem); s != nil {
			m.SubExposure, _ = strconv.ParseFloat(s[1], 64)
		}
	}

	if s := reFilter.FindStringSubmatch(stem); s != nil {
		switch k := strings.ToLower(s[1]); k {
		case "ircut", "ir-cut":
			m.Filter = "IR-cut"
		default:
			m.Filter = strings.ToUpper(s[1])
		}
	}

	if s := reDate.FindStringSubmatch(stem); s != nil {
		m.Date = s[1] + "-" + s[2] + "-" + s[3]
	}
	return m
}

// ScopeLabel drops the serial Seestar firmware appends to the telescope name
// ("S50 Pro_8dc7900b" → "S50 Pro").
func ScopeLabel(telescope string) string {
	if s := reSerial.FindStringSubmatch(telescope); s != nil {
		return s[1]
	}
	return telescope
}

// Details is the capture information shown with a picture.
type Details struct {
	Telescope string  `json:"telescope"`
	Filter    string  `json:"filter"`
	Date      string  `json:"date"` // "2006-01-02", or empty
	Subs      int     `json:"subs"` // stacked subs, when known
	SubExp    float64 `json:"subExposure"`
	// Exposure is the total integration in seconds, when known.
	Exposure float64 `json:"exposure"`
}

// Describe combines what the file name says with what the indexer read from
// the file (telescope and filter headers, DATE-OBS, EXPTIME); the name wins.
func Describe(fileName, telescope, filter, dateObs string, expTime float64) Details {
	n := ParseName(fileName)
	d := Details{Telescope: n.Telescope, Filter: n.Filter, Date: n.Date, Subs: n.Subs, SubExp: n.SubExposure}
	if d.Telescope == "" && telescope != "" {
		d.Telescope = ScopeLabel(telescope)
	}
	if d.Filter == "" {
		d.Filter = filter
	}
	if d.Date == "" {
		if s := reDateObs.FindStringSubmatch(dateObs); s != nil {
			d.Date = s[1] + "-" + s[2] + "-" + s[3]
		}
	}
	switch {
	case d.Subs > 0 && d.SubExp > 0:
		d.Exposure = float64(d.Subs) * d.SubExp
	case d.SubExp > 0:
		d.Exposure = d.SubExp
	case expTime > 0:
		d.Exposure = expTime
	}
	return d
}
