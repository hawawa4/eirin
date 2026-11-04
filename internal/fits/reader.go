package fits

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	fitsio "codeberg.org/astrogo/fitsio"
)

// ── Primary HDU reader ───────────────────────────────────────────────────────
//
// fitsio.Open reads every HDU's data into memory before returning, so asking
// it for a header pulls the whole file (50–200 MB for a stack) off the NAS.
// Here the header blocks are read on their own and handed to fitsio with
// NAXIS forced to 0, so it parses the cards exactly as before but has no data
// to load. Pixels are then streamed straight from the file into float32.

const (
	fitsBlockSize = 2880
	fitsCardSize  = 80
	// maxHeaderBlocks bounds the header scan for files that aren't FITS at all.
	maxHeaderBlocks = 1000
)

// primaryHeader is the primary HDU's header plus where its data starts.
type primaryHeader struct {
	hdr        *fitsio.Header
	bitpix     int
	axes       []int // the real axes (hdr.Axes() is empty: NAXIS was zeroed)
	dataOffset int64
}

// readPrimaryHeader reads only the primary header of the FITS file at path.
func readPrimaryHeader(path string) (*primaryHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	ph, err := decodePrimaryHeader(bufio.NewReaderSize(f, fitsBlockSize*4))
	if err != nil {
		return nil, fmt.Errorf("parse FITS %s: %w", path, err)
	}
	return ph, nil
}

func decodePrimaryHeader(r io.Reader) (*primaryHeader, error) {
	var raw []byte
	block := make([]byte, fitsBlockSize)
	naxis := -1
	ended := false
	for n := 0; !ended; n++ {
		if n >= maxHeaderBlocks {
			return nil, fmt.Errorf("no END card in the first %d header blocks", maxHeaderBlocks)
		}
		if _, err := io.ReadFull(r, block); err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		if n == 0 && !bytes.HasPrefix(block, []byte("SIMPLE  =")) {
			return nil, fmt.Errorf("not a FITS file")
		}
		for i := 0; i < fitsBlockSize; i += fitsCardSize {
			card := block[i : i+fitsCardSize]
			switch strings.TrimRight(string(card[:8]), " ") {
			case "NAXIS":
				v, err := strconv.Atoi(strings.TrimSpace(string(card[10:30])))
				if err != nil {
					return nil, fmt.Errorf("bad NAXIS card: %q", card)
				}
				naxis = v
				// Hide the data unit from fitsio; the real axes are kept below.
				copy(card, fmt.Sprintf("%-8s= %20d%-50s", "NAXIS", 0, ""))
			case "END":
				ended = true
			}
		}
		raw = append(raw, block...)
	}
	if naxis < 0 {
		return nil, fmt.Errorf("missing NAXIS card")
	}

	ff, err := fitsio.Open(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer ff.Close()
	hdu := ff.HDU(0)
	if hdu == nil {
		return nil, fmt.Errorf("no primary HDU")
	}
	hdr := hdu.Header()
	axes := make([]int, naxis)
	for i := range axes {
		axes[i] = cardInt(hdr, 0, fmt.Sprintf("NAXIS%d", i+1))
		if axes[i] < 0 {
			return nil, fmt.Errorf("negative NAXIS%d", i+1)
		}
	}
	return &primaryHeader{hdr: hdr, bitpix: hdr.Bitpix(), axes: axes, dataOffset: int64(len(raw))}, nil
}

// pixelCount is the number of values in the data unit (0 without axes).
func (ph *primaryHeader) pixelCount() int {
	if len(ph.axes) == 0 {
		return 0
	}
	n := 1
	for _, d := range ph.axes {
		n *= d
	}
	return n
}

// readPixels reads the primary image of path as float32 with BSCALE/BZERO
// applied, in FITS order (axis 1 fastest, rows bottom-up, planes last).
func readPixels(path string) (*primaryHeader, []float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	br := bufio.NewReaderSize(f, 1<<20)
	ph, err := decodePrimaryHeader(br)
	if err != nil {
		return nil, nil, fmt.Errorf("parse FITS %s: %w", path, err)
	}
	px, err := decodePixels(br, ph.bitpix, ph.pixelCount(),
		cardF64(ph.hdr, 1, "BSCALE"), cardF64(ph.hdr, 0, "BZERO"))
	if err != nil {
		return nil, nil, fmt.Errorf("read pixels of %s: %w", path, err)
	}
	return ph, px, nil
}

// decodePixels converts n big-endian values of type bitpix to float32,
// applying v*bscale + bzero. BITPIX 8 is unsigned, as the standard says.
func decodePixels(r io.Reader, bitpix, n int, bscale, bzero float64) ([]float32, error) {
	if n <= 0 {
		return nil, fmt.Errorf("image has zero pixels")
	}
	size := bitpix / 8
	if size < 0 {
		size = -size
	}
	var conv func(b []byte) float64
	switch bitpix {
	case 8:
		conv = func(b []byte) float64 { return float64(b[0]) }
	case 16:
		conv = func(b []byte) float64 { return float64(int16(binary.BigEndian.Uint16(b))) }
	case 32:
		conv = func(b []byte) float64 { return float64(int32(binary.BigEndian.Uint32(b))) }
	case 64:
		conv = func(b []byte) float64 { return float64(int64(binary.BigEndian.Uint64(b))) }
	case -32:
		conv = func(b []byte) float64 { return float64(math.Float32frombits(binary.BigEndian.Uint32(b))) }
	case -64:
		conv = func(b []byte) float64 { return math.Float64frombits(binary.BigEndian.Uint64(b)) }
	default:
		return nil, fmt.Errorf("unsupported BITPIX %d", bitpix)
	}

	out := make([]float32, n)
	identity := bscale == 1 && bzero == 0
	buf := make([]byte, (1<<20)/size*size)
	for i := 0; i < n; {
		chunk := buf[:min(len(buf), (n-i)*size)]
		if _, err := io.ReadFull(r, chunk); err != nil {
			return nil, err
		}
		if bitpix == -32 && identity {
			// The common case (stacks, processed images): no float64 detour.
			for o := 0; o < len(chunk); o += 4 {
				out[i] = math.Float32frombits(binary.BigEndian.Uint32(chunk[o:]))
				i++
			}
			continue
		}
		for o := 0; o < len(chunk); o += size {
			out[i] = float32(conv(chunk[o:o+size])*bscale + bzero)
			i++
		}
	}
	return out, nil
}
