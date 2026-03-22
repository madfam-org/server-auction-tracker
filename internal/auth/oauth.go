package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// OAuthFlow handles the OAuth2 authorization code flow with Janua.
type OAuthFlow struct {
	clientID      string
	clientSecret  string
	authURL       string
	tokenURL      string
	redirectURL   string
	validator     *Validator
	sessionSecret []byte
}

// NewOAuthFlow creates an OAuth flow handler.
func NewOAuthFlow(clientID, clientSecret, authURL, tokenURL, redirectURL string, validator *Validator, sessionSecret []byte) *OAuthFlow {
	return &OAuthFlow{
		clientID:      clientID,
		clientSecret:  clientSecret,
		authURL:       authURL,
		tokenURL:      tokenURL,
		redirectURL:   redirectURL,
		validator:     validator,
		sessionSecret: sessionSecret,
	}
}

// LoginHandler redirects the user to Janua's authorize endpoint.
func (o *OAuthFlow) LoginHandler(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Store state in a short-lived httpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "ds_oauth_state",
		Value:    state,
		Path:     "/auth/callback",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	params := url.Values{
		"response_type": {"code"},
		"client_id":     {o.clientID},
		"redirect_uri":  {o.redirectURL},
		"scope":         {"openid profile email"},
		"state":         {state},
	}

	http.Redirect(w, r, o.authURL+"?"+params.Encode(), http.StatusFound)
}

// CallbackHandler exchanges the authorization code for tokens and sets a session cookie.
func (o *OAuthFlow) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Verify state
	stateCookie, err := r.Cookie("ds_oauth_state")
	if err != nil || stateCookie.Value == "" {
		http.Error(w, "missing state cookie", http.StatusBadRequest)
		return
	}

	stateParam := r.URL.Query().Get("state")
	if stateParam != stateCookie.Value {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "ds_oauth_state",
		Value:    "",
		Path:     "/auth/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	// Check for error from authorization server
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		log.WithFields(log.Fields{"error": errParam, "description": desc}).Warn("OAuth callback error")
		http.Error(w, fmt.Sprintf("authorization error: %s", errParam), http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	// Exchange code for tokens
	tokenResp, err := o.exchangeCode(code)
	if err != nil {
		log.WithError(err).Error("Failed to exchange authorization code")
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	// Validate the ID token
	idToken := tokenResp.IDToken
	if idToken == "" {
		idToken = tokenResp.AccessToken
	}

	claims, err := o.validator.ValidateToken(idToken)
	if err != nil {
		log.WithError(err).Error("ID token validation failed")
		http.Error(w, "token validation failed", http.StatusUnauthorized)
		return
	}

	if !o.validator.IsAuthorized(claims) {
		log.WithField("email", claims.Email).Warn("Unauthorized user attempted login")
		http.Error(w, "unauthorized: access requires @madfam.io email with operator role", http.StatusForbidden)
		return
	}

	// Set session cookie with the access token (signed with HMAC)
	accessToken := tokenResp.AccessToken
	if accessToken == "" {
		accessToken = idToken
	}

	signedCookie := o.signCookie(accessToken)
	http.SetCookie(w, &http.Cookie{
		Name:     "ds_session",
		Value:    signedCookie,
		Path:     "/",
		MaxAge:   86400, // 24 hours
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	log.WithField("email", claims.Email).Info("User authenticated via SSO")
	http.Redirect(w, r, "/", http.StatusFound)
}

// LogoutHandler clears the session cookie.
func (o *OAuthFlow) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "ds_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSONResp(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// MeHandler returns the current user's info from their session.
func (o *OAuthFlow) MeHandler(w http.ResponseWriter, r *http.Request) {
	token, err := o.extractSessionToken(r)
	if err != nil {
		writeJSONResp(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	claims, err := o.validator.ValidateToken(token)
	if err != nil {
		writeJSONResp(w, http.StatusUnauthorized, map[string]string{"error": "invalid session"})
		return
	}

	writeJSONResp(w, http.StatusOK, map[string]interface{}{
		"email": claims.Email,
		"name":  claims.Name,
		"roles": claims.Roles,
	})
}

// ExtractSessionToken extracts and verifies the access token from the ds_session cookie.
func (o *OAuthFlow) extractSessionToken(r *http.Request) (string, error) {
	cookie, err := r.Cookie("ds_session")
	if err != nil {
		return "", fmt.Errorf("no session cookie")
	}
	return o.verifyCookie(cookie.Value)
}

// ExtractToken tries to get a valid JWT from the request (cookie or Bearer header).
// Returns the token string or empty if not found.
func (o *OAuthFlow) ExtractToken(r *http.Request) string {
	// Try session cookie first
	if token, err := o.extractSessionToken(r); err == nil {
		return token
	}

	// Try Bearer header (may be a Janua JWT)
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func (o *OAuthFlow) exchangeCode(code string) (*tokenResponse, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {o.redirectURL},
		"client_id":    {o.clientID},
	}

	req, err := http.NewRequest("POST", o.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(o.clientID, o.clientSecret)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	return &tokenResp, nil
}

func (o *OAuthFlow) signCookie(token string) string {
	mac := hmac.New(sha256.New, o.sessionSecret)
	mac.Write([]byte(token))
	sig := hex.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(token)) + "." + sig
}

func (o *OAuthFlow) verifyCookie(value string) (string, error) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid cookie format")
	}

	tokenBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid cookie encoding")
	}

	mac := hmac.New(sha256.New, o.sessionSecret)
	mac.Write(tokenBytes)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return "", fmt.Errorf("cookie signature mismatch")
	}

	return string(tokenBytes), nil
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func writeJSONResp(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
