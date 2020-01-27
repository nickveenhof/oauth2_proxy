package providers

import (
	"net/url"
	oidc "github.com/coreos/go-oidc"
)

// ProviderData contains information required to configure all implementations
// of OAuth2 providers
type ProviderData struct {
	ProviderName      string
	ClientID          string
	ClientSecret      string
	LoginURL          *url.URL
	RedeemURL         *url.URL
	ProfileURL        *url.URL
	ProtectedResource *url.URL
	ValidateURL       *url.URL
	Scope             string
	ApprovalPrompt    string
    JwtBearerVerifiers []*oidc.IDTokenVerifier
}

// Data returns the ProviderData
func (p *ProviderData) Data() *ProviderData { return p }
