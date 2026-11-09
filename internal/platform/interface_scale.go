package platform

import (
	"fmt"
	"math"
	"sync"
)

var interfaceScaleHook struct {
	sync.RWMutex
	apply func(float64)
}

func validInterfaceScale(scale float64) bool {
	return !math.IsNaN(scale) && !math.IsInf(scale, 0) && (scale == 0 || scale >= 0.75 && scale <= 2)
}

func SetInterfaceScaleHandler(apply func(float64)) {
	interfaceScaleHook.Lock()
	interfaceScaleHook.apply = apply
	interfaceScaleHook.Unlock()
}

func (p *PlatformService) GetInterfaceScale() (float64, error) {
	var scale float64
	err := p.withSettingsRead(func(s *AppSettings) error { scale = s.InterfaceScale; return nil })
	return scale, err
}

func (p *PlatformService) SetInterfaceScale(scale float64) error {
	if !validInterfaceScale(scale) {
		return fmt.Errorf("interface scale must be automatic (0) or between 0.75 and 2")
	}
	if err := p.withSettingsWrite(func(s *AppSettings) error { s.InterfaceScale = scale; return nil }); err != nil {
		return err
	}
	interfaceScaleHook.RLock()
	apply := interfaceScaleHook.apply
	interfaceScaleHook.RUnlock()
	if apply != nil {
		apply(scale)
	}
	return nil
}
