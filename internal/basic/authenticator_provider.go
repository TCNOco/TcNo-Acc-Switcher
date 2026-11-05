package basic

import "TcNo-Acc-Switcher/internal/security"

func (b *BasicService) GetAuthenticatorProvider(platformKey string) (string, error) {
	if err := security.RequireUnlocked(); err != nil {
		return "", err
	}
	d, _, err := readDescriptor(platformKey)
	return d.Extras.AuthenticatorProvider, err
}
