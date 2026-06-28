package platform

import (
	"os"

	"TcNo-Acc-Switcher/internal/security"
)

func (p *PlatformService) GetUserDataLocation() (string, error) {
	return GetUserDataLocation()
}

func (p *PlatformService) GetPortableUserDataLocation() (string, error) {
	return GetPortableUserDataLocation()
}

func (p *PlatformService) MoveUserDataTo(destination string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return MoveUserDataTo(destination)
}

func (p *PlatformService) MoveUserDataPortable() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return MoveUserDataPortable()
}

func (p *PlatformService) GetDefaultUserDataLocation() (string, error) {
	return GetDefaultUserDataLocation()
}

func (p *PlatformService) MoveUserDataAppData() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return MoveUserDataAppData()
}

func (p *PlatformService) OpenUserDataFolder() error {
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	dir, err := GetUserDataLocation()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return OpenPathInFileManager(dir)
}
