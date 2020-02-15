package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pusher/oauth2_proxy/pkg/apis/sessions"
	"github.com/pusher/oauth2_proxy/pkg/encryption"
	"github.com/pusher/oauth2_proxy/pkg/logger"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

// Redeem provides a default implementation of the OAuth2 token redemption process
func (p *ProviderData) Redeem(redirectURL, code string) (s *sessions.SessionState, err error) {
	if code == "" {
		err = errors.New("missing code")
		return
	}

	params := url.Values{}
	params.Add("redirect_uri", redirectURL)
	params.Add("client_id", p.ClientID)
	params.Add("client_secret", p.ClientSecret)
	params.Add("code", code)
	params.Add("grant_type", "authorization_code")
	if p.ProtectedResource != nil && p.ProtectedResource.String() != "" {
		params.Add("resource", p.ProtectedResource.String())
	}

	var req *http.Request
	req, err = http.NewRequest("POST", p.RedeemURL.String(), bytes.NewBufferString(params.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return
	}

	if resp.StatusCode != 200 {
		err = fmt.Errorf("got %d from %q %s", resp.StatusCode, p.RedeemURL.String(), body)
		return
	}

	// blindly try json and x-www-form-urlencoded
	var jsonResponse struct {
		AccessToken string `json:"access_token"`
	}
	err = json.Unmarshal(body, &jsonResponse)
	if err == nil {
		s = &sessions.SessionState{
			AccessToken: jsonResponse.AccessToken,
		}
		return
	}

	var v url.Values
	v, err = url.ParseQuery(string(body))
	if err != nil {
		return
	}
	if a := v.Get("access_token"); a != "" {
		s = &sessions.SessionState{AccessToken: a, CreatedAt: time.Now()}
	} else {
		err = fmt.Errorf("no access token found %s", body)
	}
	return
}

// GetLoginURL with typical oauth parameters
func (p *ProviderData) GetLoginURL(redirectURI, state string) string {
	var a url.URL
	a = *p.LoginURL
	params, _ := url.ParseQuery(a.RawQuery)
	params.Set("redirect_uri", redirectURI)
	params.Set("approval_prompt", p.ApprovalPrompt)
	params.Add("scope", p.Scope)
	params.Set("client_id", p.ClientID)
	params.Set("response_type", "code")
	params.Add("state", state)
	a.RawQuery = params.Encode()
	return a.String()
}

// CookieForSession serializes a session state for storage in a cookie
func (p *ProviderData) CookieForSession(s *sessions.SessionState, c *encryption.Cipher) (string, error) {
	return s.EncodeSessionState(c)
}

// SessionFromCookie deserializes a session from a cookie value
func (p *ProviderData) SessionFromCookie(v string, c *encryption.Cipher) (s *sessions.SessionState, err error) {
	return sessions.DecodeSessionState(v, c)
}

// GetEmailAddress returns the Account email address
func (p *ProviderData) GetEmailAddress(s *sessions.SessionState) (string, error) {
	return "", errors.New("not implemented")
}

// GetUserName returns the Account username
func (p *ProviderData) GetUserName(s *sessions.SessionState) (string, error) {
	return "", errors.New("not implemented")
}

// ValidateGroup validates that the provided email exists in the configured provider
// email group(s).
func (p *ProviderData) ValidateGroup(email string) bool {
	return true
}

// ValidateSessionState validates the AccessToken
func (p *ProviderData) ValidateSessionState(s *sessions.SessionState) bool {
	return validateToken(p, s.AccessToken, nil)
}

// RefreshSessionIfNeeded should refresh the user's session if required and
// do nothing if a refresh is not required
func (p *ProviderData) RefreshSessionIfNeeded(s *sessions.SessionState) (bool, error) {
	return false, nil
}

// GetJwtSession loads a session based on a JWT token in the authorization header.
func (p *ProviderData) GetJwtSession(rawBearerToken string) (*sessions.SessionState, error) {
	ctx := context.Background()
	var session *sessions.SessionState

	if p == nil {
		return nil, fmt.Errorf("No JwtBearerVerifiers found")
	}

	for _, verifier := range p.JwtBearerVerifiers {
		bearerToken, err := verifier.Verify(ctx, rawBearerToken)

		if err != nil {
			logger.Printf("failed to verify bearer token: %v", err)
			continue
		}

		var claims struct {
			Subject  string `json:"sub"`
			Email    string `json:"email"`
			Verified *bool  `json:"email_verified"`
		}

		if err := bearerToken.Claims(&claims); err != nil {
			return nil, fmt.Errorf("failed to parse bearer token claims: %v", err)
		}

		if claims.Email == "" {
			claims.Email = claims.Subject
		}

		if claims.Verified != nil && !*claims.Verified {
			return nil, fmt.Errorf("email in id_token (%s) isn't verified", claims.Email)
		}

		session = &sessions.SessionState{
			AccessToken:  rawBearerToken,
			IDToken:      rawBearerToken,
			RefreshToken: "",
			ExpiresOn:    bearerToken.Expiry,
			Email:        claims.Email,
			User:         claims.Email,
		}
		return session, nil
	}
	return nil, fmt.Errorf("failed to process the raw bearer token or there were no bearer verifiers present")
}
