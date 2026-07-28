package protocol

import "time"

// PlatformType is Steam's authentication token platform type.
type PlatformType uint32

const (
	PlatformSteamClient PlatformType = 1
	PlatformWebBrowser  PlatformType = 2
	PlatformMobileApp   PlatformType = 3
)

// AppType identifies a Steam mobile application.
type AppType uint32

const (
	AppTypeUnknown     AppType = 0
	AppTypeSteamMobile AppType = 1
	AppTypeSteamChat   AppType = 2
)

// Persistence selects whether Steam may issue a renewable session. Invalid is
// used only when Steam does not report the requestor's preference.
type Persistence int32

const (
	PersistenceInvalid    Persistence = -1
	PersistenceEphemeral  Persistence = 0
	PersistencePersistent Persistence = 1
)

// SecurityHistory describes whether the requesting location has authenticated
// successfully before.
type SecurityHistory uint32

const (
	SecurityHistoryInvalid        SecurityHistory = 0
	SecurityHistoryUsedPreviously SecurityHistory = 1
	SecurityHistoryNoPriorHistory SecurityHistory = 2
)

// RenewalType controls refresh-token renewal during access-token generation.
type RenewalType uint32

const (
	RenewalNone  RenewalType = 0
	RenewalAllow RenewalType = 1
)

// ChallengeType is one challenge Steam permits for an authentication session.
type ChallengeType uint32

const (
	ChallengeNone               ChallengeType = 1
	ChallengeEmailCode          ChallengeType = 2
	ChallengeDeviceCode         ChallengeType = 3
	ChallengeDeviceConfirmation ChallengeType = 4
	ChallengeEmailConfirmation  ChallengeType = 5
	ChallengeMachineToken       ChallengeType = 6
	ChallengeLegacyMachineAuth  ChallengeType = 7
)

// AuthResultState is the next action required after an authentication call.
type AuthResultState string

const (
	AuthResultWaiting           AuthResultState = "waiting"
	AuthResultChallengeRequired AuthResultState = "challenge_required"
	AuthResultAgreementRequired AuthResultState = "agreement_required"
	AuthResultAuthorized        AuthResultState = "authorized"
	AuthResultChallengeAccepted AuthResultState = "challenge_accepted"
	AuthResultChallengeDenied   AuthResultState = "challenge_denied"
	AuthResultTokenIssued       AuthResultState = "token_issued"
)

// DeviceDetails identifies the client that requests authentication.
type DeviceDetails struct {
	FriendlyName     string
	Platform         PlatformType
	OSType           int32
	GamingDeviceType uint32
	ClientCount      uint32
	MachineID        []byte
	App              AppType
}

// BeginCredentialsRequest starts a session after the password has been RSA
// encrypted with Steam's current account key.
type BeginCredentialsRequest struct {
	DeviceFriendlyName  string
	AccountName         string
	EncryptedPassword   string
	EncryptionTimestamp uint64
	RememberLogin       bool
	Platform            PlatformType
	Persistence         Persistence
	WebsiteID           string
	Device              DeviceDetails
	GuardData           string
	Language            uint32
	QoSLevel            int32
}

// AllowedChallenge is one challenge returned by Steam. AssociatedMessage may
// contain an email hint and must not be logged.
type AllowedChallenge struct {
	Type              ChallengeType
	AssociatedMessage string
}

// AuthSession holds the server identifiers required for code submission and
// polling. Its request ID is kept private to prevent accidental serialization.
type AuthSession struct {
	id           string
	clientID     uint64
	requestID    []byte
	steamID      uint64
	pollInterval time.Duration
	challenges   []AllowedChallenge
}

func (s AuthSession) ID() string                  { return s.id }
func (s AuthSession) ClientID() uint64            { return s.clientID }
func (s AuthSession) SteamID() uint64             { return s.steamID }
func (s AuthSession) PollInterval() time.Duration { return s.pollInterval }
func (s AuthSession) Challenges() []AllowedChallenge {
	return append([]AllowedChallenge(nil), s.challenges...)
}

// Destroy invalidates the session and wipes its mutable server request ID.
// Copies of AuthSession share that request ID and become invalid at the same time.
func (s *AuthSession) Destroy() {
	if s == nil {
		return
	}
	wipeBytes(s.requestID)
	s.id = ""
	s.clientID = 0
	s.requestID = nil
	s.steamID = 0
	s.pollInterval = 0
	s.challenges = nil
}

// BeginCredentialsResult reports the session and its next required action.
type BeginCredentialsResult struct {
	State         AuthResultState
	Session       AuthSession
	AgreementURL  string
	ServerMessage string
}

// SteamGuardCodeRequest submits a five-character email or device code.
type SteamGuardCodeRequest struct {
	Session AuthSession
	Code    string
	Type    ChallengeType
}

// ChallengeResult reports whether a submitted challenge was accepted or an
// agreement page must be completed first.
type ChallengeResult struct {
	State        AuthResultState
	AgreementURL string
}

// PollResult contains newly issued credentials. Tokens and GuardData are
// secrets and must move directly into protected storage.
type PollResult struct {
	State                AuthResultState
	Session              AuthSession
	RefreshToken         string
	AccessToken          string
	HadRemoteInteraction bool
	AccountName          string
	GuardData            string
	ChallengeURL         string
	AgreementURL         string
}

// GenerateAccessTokenRequest exchanges a refresh token for an app token.
type GenerateAccessTokenRequest struct {
	SteamID      uint64
	RefreshToken string
	Renewal      RenewalType
}

// TokenResult contains tokens issued by GenerateAccessTokenForApp.
type TokenResult struct {
	State        AuthResultState
	AccessToken  string
	RefreshToken string
}

// AuthSessionInfoRequest identifies an authentication request to inspect with
// a MobileApp access token.
type AuthSessionInfoRequest struct {
	AccessToken string
	ClientID    uint64
}

// AuthSessionInfo contains bounded requestor metadata shown before a QR login
// is approved. Empty location fields mean Steam did not provide them.
type AuthSessionInfo struct {
	IPAddress                 string
	GeoLocation               string
	City                      string
	State                     string
	Country                   string
	Platform                  PlatformType
	DeviceFriendlyName        string
	Version                   int32
	LoginHistory              SecurityHistory
	RequestorLocationMismatch bool
	HighUsageLogin            bool
	RequestedPersistence      Persistence
	DeviceTrust               int32
	App                       AppType
}

// MobileConfirmationRequest approves or denies a QR authentication challenge.
type MobileConfirmationRequest struct {
	AccessToken string
	Version     int32
	ClientID    uint64
	SteamID     uint64
	Signature   []byte
	Confirm     bool
	Persistence Persistence
}
