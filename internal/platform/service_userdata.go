package platform

import (
	"os"
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
	return MoveUserDataTo(destination)
}

func (p *PlatformService) MoveUserDataPortable() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return MoveUserDataPortable()
}

func (p *PlatformService) GetDefaultUserDataLocation() (string, error) {
	return GetDefaultUserDataLocation()
}

func (p *PlatformService) MoveUserDataAppData() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return MoveUserDataAppData()
}

func (p *PlatformService) OpenUserDataFolder() error {
	dir, err := GetUserDataLocation()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return OpenPathInFileManager(dir)
}
