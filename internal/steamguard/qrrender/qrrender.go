// Package qrrender draws a Steam sign-in challenge URL as a QR code.
//
// The sibling qrimage package reads QR codes out of screenshots; this one writes
// them. Both sit on github.com/piglig/go-qr, which is pure Go and does its own
// encoding, so nothing here reaches the network or the filesystem.
package qrrender

import (
	"encoding/base64"
	"errors"
	"fmt"
	"image/color"
	"strings"

	goqr "github.com/piglig/go-qr"
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
)

var ErrNothingToEncode = errors.New("qrrender: nothing to encode")

// SVGDataURI returns the QR code for text as a data URI ready for an <img> src.
//
// SVG rather than PNG because the code is drawn at whatever size the screen has
// room for, and a resampled PNG of a QR code is exactly the thing phone cameras
// struggle with.
//
// Painted white rather than left transparent. go-qr writes a transparent light
// colour as fill="" on the background rect, which is not a valid presentation
// attribute value: the browser drops it, fill falls back to its initial value -
// black - and the rect covers the whole code. A scanner wants a light quiet zone
// anyway, so white is what this should have asked for to begin with.
func SVGDataURI(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > maxTextBytes {
		return "", ErrNothingToEncode
	}
	// Medium correction: enough to survive a little glare or a fingerprint on the
	// screen, without the module count that makes a code harder to focus on.
	code, err := goqr.EncodeText(text, goqr.Medium)
	if err != nil {
		return "", fmt.Errorf("qrrender: encode: %w", err)
	}
	config := goqr.NewQrCodeImgConfig(scale, border,
		goqr.WithLight(color.White),
		goqr.WithDark(color.Black),
	)
	svg, err := code.ToSVGBytes(config)
	if err != nil {
		return "", fmt.Errorf("qrrender: render: %w", err)
	}
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(svg), nil
}
