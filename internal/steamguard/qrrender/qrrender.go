// Package qrrender draws a Steam sign-in challenge URL as a QR code.
//
// Encoding is pure Go (github.com/makiuchi-d/gozxing), so nothing here reaches
// the network or the filesystem.
package qrrender

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/makiuchi-d/gozxing/qrcode/decoder"
)

const (
	// scale is in SVG user units per module, so it decides the viewBox rather
	// than the drawn size: the page scales the image with CSS.
	scale = 8
	// border is the quiet zone, in modules. Four is the QR specification's
	// minimum, and a code without it is unreliable to scan against a page
	// background.
	border = 4
	// maxTextBytes is well past a Steam challenge URL, which is around forty
	// characters. It only stops an unbounded string reaching the encoder.
	maxTextBytes = 512
	// maxModules is version 40, the largest symbol the specification defines.
	// A challenge URL is nowhere near it; this only bounds the SVG a malformed
	// encode could ask for.
	maxModules = 177
)

var ErrNothingToEncode = errors.New("qrrender: nothing to encode")

// SVGDataURI returns the QR code for text as a data URI ready for an <img> src.
//
// SVG rather than PNG because the code is drawn at whatever size the screen has
// room for, and a resampled PNG of a QR code is exactly the thing phone cameras
// struggle with.
//
// The markup is written here so both colours are always spelled out: an empty
// or invalid fill attribute falls back to the property's initial value, which
// turns the whole code black.
func SVGDataURI(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > maxTextBytes {
		return "", ErrNothingToEncode
	}
	// Medium correction survives a little glare or a fingerprint on the screen
	// without the module count that makes a code harder to focus on. The zero
	// size asks for the symbol's natural module count, and the quiet zone is
	// drawn below, so the writer adds none of its own.
	matrix, err := qrcode.NewQRCodeWriter().Encode(text, gozxing.BarcodeFormat_QR_CODE, 0, 0,
		map[gozxing.EncodeHintType]interface{}{
			gozxing.EncodeHintType_ERROR_CORRECTION: decoder.ErrorCorrectionLevel_M,
			gozxing.EncodeHintType_MARGIN:           0,
		})
	if err != nil {
		return "", fmt.Errorf("qrrender: encode: %w", err)
	}
	svg, err := svgFromMatrix(matrix)
	if err != nil {
		return "", err
	}
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg)), nil
}

func svgFromMatrix(matrix *gozxing.BitMatrix) (string, error) {
	if matrix == nil {
		return "", ErrNothingToEncode
	}
	modules := matrix.GetWidth()
	if modules <= 0 || modules != matrix.GetHeight() || modules > maxModules {
		return "", fmt.Errorf("qrrender: unexpected symbol size %dx%d", matrix.GetWidth(), matrix.GetHeight())
	}
	dimension := strconv.Itoa((modules + border*2) * scale)

	var out strings.Builder
	out.Grow(256 + modules*modules*20)
	out.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" version="1.1" viewBox="0 0 `)
	out.WriteString(dimension)
	out.WriteByte(' ')
	out.WriteString(dimension)
	out.WriteString(`" stroke="none">`)
	out.WriteString(`<rect width="`)
	out.WriteString(dimension)
	out.WriteString(`" height="`)
	out.WriteString(dimension)
	out.WriteString(`" fill="#FFFFFF"/>`)
	out.WriteString(`<path fill="#000000" d="`)
	for y := 0; y < modules; y++ {
		for x := 0; x < modules; x++ {
			if !matrix.Get(x, y) {
				continue
			}
			out.WriteByte('M')
			out.WriteString(strconv.Itoa((x + border) * scale))
			out.WriteByte(',')
			out.WriteString(strconv.Itoa((y + border) * scale))
			out.WriteByte('h')
			out.WriteString(strconv.Itoa(scale))
			out.WriteByte('v')
			out.WriteString(strconv.Itoa(scale))
			out.WriteByte('h')
			out.WriteString(strconv.Itoa(-scale))
			out.WriteByte('z')
		}
	}
	out.WriteString(`"/></svg>`)
	return out.String(), nil
}
