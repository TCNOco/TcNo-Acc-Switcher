package winutil

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// RenderSVGRequest is emitted to the frontend to rasterize SVG when oksvg fails.
type RenderSVGRequest struct {
	ID   string `json:"id"`
	SVG  string `json:"svg"`
	Size int    `json:"size"`
}

const svgRenderRequestEvent = "shortcut:render-svg-request"

var (
	svgWaitMu sync.Mutex
	svgWait   = map[string]chan renderOutcome{}
)

type renderOutcome struct {
	png []byte
	err error
}

func init() {
	application.RegisterEvent[RenderSVGRequest](svgRenderRequestEvent)
}

// DeliverSVGRenderResult completes a pending RequestSVGRenderViaWails (called from Wails service when frontend finishes).
func DeliverSVGRenderResult(id, pngBase64, errMsg string) {
	svgWaitMu.Lock()
	ch, ok := svgWait[id]
	delete(svgWait, id)
	svgWaitMu.Unlock()
	if !ok || ch == nil {
		return
	}
	var out renderOutcome
	switch {
	case errMsg != "":
		out.err = &svgRenderUserError{msg: errMsg}
	case pngBase64 != "":
		b, err := base64.StdEncoding.DecodeString(pngBase64)
		if err != nil {
			out.err = err
		} else {
			out.png = b
		}
	default:
		out.err = &svgRenderUserError{msg: "empty png"}
	}
	select {
	case ch <- out:
	default:
	}
}

type svgRenderUserError struct {
	msg string
}

func (e *svgRenderUserError) Error() string { return e.msg }

// RequestSVGRenderViaWails asks the WebView to rasterize SVG to PNG; returns error if no app or timeout.
func RequestSVGRenderViaWails(svg string, size int, timeout time.Duration) ([]byte, error) {
	app := application.Get()
	if app == nil {
		return nil, &svgRenderUserError{msg: "no application"}
	}
	var idRand [16]byte
	if _, err := rand.Read(idRand[:]); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(idRand[:])
	ch := make(chan renderOutcome, 1)
	svgWaitMu.Lock()
	svgWait[id] = ch
	svgWaitMu.Unlock()

	defer func() {
		svgWaitMu.Lock()
		if cur, ok := svgWait[id]; ok && cur == ch {
			delete(svgWait, id)
		}
		svgWaitMu.Unlock()
	}()

	app.Event.Emit(svgRenderRequestEvent, RenderSVGRequest{ID: id, SVG: svg, Size: size})
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	select {
	case out := <-ch:
		return out.png, out.err
	case <-time.After(timeout):
		return nil, &svgRenderUserError{msg: "svg render timeout"}
	}
}
