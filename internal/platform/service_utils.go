package platform

import (
	"TcNo-Acc-Switcher/internal/stats"
)

func (p *PlatformService) GetPlatformSettings(platformKey string) (PlatformSettings, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return LoadPlatformSettings(platformKey)
}

func (p *PlatformService) SavePlatformSettings(platformKey string, s PlatformSettings) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return SavePlatformSettings(platformKey, s)
}

func (p *PlatformService) ResetPlatformSettings(platformKey string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return resetPlatformJSONToDefaults(platformKey)
}

func (p *PlatformService) GetStatsReport() (StatsReport, error) {
	p.mu.RLock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		p.mu.RUnlock()
		return StatsReport{}, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		p.mu.RUnlock()
		return StatsReport{}, err
	}
	share := s.StatsShare
	p.mu.RUnlock()

	data, err := stats.GetReportData()
	if err != nil {
		return StatsReport{}, err
	}
	return assembleStatsReport(data, share), nil
}

func (p *PlatformService) ResetStatistics() error {
	return stats.ResetStatistics()
}

func (p *PlatformService) StatsRecordPageVisit(pagePath string) error {
	return stats.RecordPageVisit(pagePath)
}

func (p *PlatformService) StatsAddPageTime(pagePath string, seconds int) error {
	return stats.AddPageTime(pagePath, seconds)
}
