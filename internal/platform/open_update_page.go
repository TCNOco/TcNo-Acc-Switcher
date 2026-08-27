package platform

const updateDownloadPageURL = "https://github.com/TCNOco/TcNo-Acc-Switcher/releases/latest"

// OpenUpdateDownloadPage opens the latest GitHub release page in the default browser.
func (p *PlatformService) OpenUpdateDownloadPage() error {
	return OpenURL(updateDownloadPageURL)
}
