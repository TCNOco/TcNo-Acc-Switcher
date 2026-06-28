package security

type SecurityService struct{}

func NewService() *SecurityService {
	return &SecurityService{}
}

func (s *SecurityService) GetSecurityStatus() (Status, error) {
	return GetStatus()
}

func (s *SecurityService) SetAppPassword(password string) error {
	return SetAppPassword(password)
}

func (s *SecurityService) UnlockApp(password string) error {
	return UnlockApp(password)
}

func (s *SecurityService) RemoveAppPassword(password string) error {
	return RemoveAppPassword(password)
}

func (s *SecurityService) EnableSavedAccountEncryption(password string) error {
	return EnableSavedAccountEncryption(password)
}

func (s *SecurityService) DisableSavedAccountEncryption(password string) error {
	return DisableSavedAccountEncryption(password)
}

func (s *SecurityService) ResetPasswordAndEncryptedSessions() error {
	return ResetPasswordAndEncryptedSessions()
}

func (s *SecurityService) ListQuarantines() ([]QuarantineInfo, error) {
	return ListQuarantines()
}

func (s *SecurityService) DeleteQuarantine(id string) error {
	return DeleteQuarantine(id)
}

func (s *SecurityService) RetryQuarantineImport(id, password string) error {
	return RetryQuarantineImport(id, password)
}
