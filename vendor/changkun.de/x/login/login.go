// Copyright (c) 2021 Changkun Ou <hi@changkun.de>. All Rights Reserved.
// Unauthorized using, copying, modifying and distributing, via any
// medium is strictly prohibited.

package login

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
)

var (
	// AuthEndpoint is the login authorization endpoint.
	AuthEndpoint = "https://login.changkun.de/auth"
	// VerifyEndpoint is the login verify endpoint.
	VerifyEndpoint = "https://login.changkun.de/verify"
)

var (
	ErrBadRequest   = errors.New("bad request")
	ErrUnauthorized = errors.New("unauthorized login")
)

// Verify checks if the given login token is valid or not.
func Verify(token string) error {
	b, _ := json.Marshal(struct {
		Token string `json:"token"`
	}{
		Token: token,
	})
	br := bytes.NewReader(b)

	resp, err := http.DefaultClient.Post(VerifyEndpoint, "application/json", br)
	if err != nil {
		return ErrBadRequest
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ErrUnauthorized
	}

	return nil
}

// AuthHandler offers the common abstraction for login with credentials.
type AuthHandler struct {
	creds func(w http.ResponseWriter, r *http.Request) (string, string)
}

// NewAuthHandler creates a new authentication handler.
func NewAuthHandler(creds func(w http.ResponseWriter, r *http.Request) (string, string)) *AuthHandler {
	return &AuthHandler{creds}
}

// Handle handles the login authentication requests and returns an error if failed.
func (ah *AuthHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	// 1st try: query parameter.
	token := r.URL.Query().Get("token")
	if token != "" {
		err := Verify(token)
		if errors.Is(err, ErrUnauthorized) {
			return err
		}

		// 2nd try: cookie.
		c, err := r.Cookie("auth")
		if err == nil {
			return Verify(c.Value)
		}
	}

	user, pass := ah.creds(w, r)
	b, _ := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: user, Password: pass})
	br := bytes.NewReader(b)

	resp, err := http.DefaultClient.Post(AuthEndpoint, "application/json", br)
	if err != nil {
		return ErrBadRequest
	}
	defer resp.Body.Close()

	cookies := resp.Cookies()
	if resp.StatusCode != http.StatusOK || len(cookies) == 0 {
		return ErrUnauthorized
	}
	return Verify(cookies[0].Value)
}
