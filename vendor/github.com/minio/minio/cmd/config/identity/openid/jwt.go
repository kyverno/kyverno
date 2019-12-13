/*
 * MinIO Cloud Storage, (C) 2018-2019 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package openid

import (
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	jwtgo "github.com/dgrijalva/jwt-go"
	"github.com/minio/minio/cmd/config"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/env"
	xnet "github.com/minio/minio/pkg/net"
)

// Config - OpenID Config
// RSA authentication target arguments
type Config struct {
	JWKS struct {
		URL *xnet.URL `json:"url"`
	} `json:"jwks"`
	URL          *xnet.URL `json:"url,omitempty"`
	ClaimPrefix  string    `json:"claimPrefix,omitempty"`
	DiscoveryDoc DiscoveryDoc
	ClientID     string
	publicKeys   map[string]crypto.PublicKey
	transport    *http.Transport
	closeRespFn  func(io.ReadCloser)
}

// PopulatePublicKey - populates a new publickey from the JWKS URL.
func (r *Config) PopulatePublicKey() error {
	if r.JWKS.URL == nil || r.JWKS.URL.String() == "" {
		return nil
	}
	transport := http.DefaultTransport
	if r.transport != nil {
		transport = r.transport
	}
	client := &http.Client{
		Transport: transport,
	}
	resp, err := client.Get(r.JWKS.URL.String())
	if err != nil {
		return err
	}
	defer r.closeRespFn(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}

	var jwk JWKS
	if err = json.NewDecoder(resp.Body).Decode(&jwk); err != nil {
		return err
	}

	for _, key := range jwk.Keys {
		r.publicKeys[key.Kid], err = key.DecodePublicKey()
		if err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalJSON - decodes JSON data.
func (r *Config) UnmarshalJSON(data []byte) error {
	// subtype to avoid recursive call to UnmarshalJSON()
	type subConfig Config
	var sr subConfig

	if err := json.Unmarshal(data, &sr); err != nil {
		return err
	}

	ar := Config(sr)
	if ar.JWKS.URL == nil || ar.JWKS.URL.String() == "" {
		*r = ar
		return nil
	}

	*r = ar
	return nil
}

// JWT - rs client grants provider details.
type JWT struct {
	Config
}

// GetDefaultExpiration - returns the expiration seconds expected.
func GetDefaultExpiration(dsecs string) (time.Duration, error) {
	defaultExpiryDuration := time.Duration(60) * time.Minute // Defaults to 1hr.
	if dsecs != "" {
		expirySecs, err := strconv.ParseInt(dsecs, 10, 64)
		if err != nil {
			return 0, auth.ErrInvalidDuration
		}

		// The duration, in seconds, of the role session.
		// The value can range from 900 seconds (15 minutes)
		// to 12 hours.
		if expirySecs < 900 || expirySecs > 43200 {
			return 0, auth.ErrInvalidDuration
		}

		defaultExpiryDuration = time.Duration(expirySecs) * time.Second
	}
	return defaultExpiryDuration, nil
}

func updateClaimsExpiry(dsecs string, claims map[string]interface{}) error {
	expStr := claims["exp"]
	if expStr == "" {
		return ErrTokenExpired
	}

	// No custom duration requested, the claims can be used as is.
	if dsecs == "" {
		return nil
	}

	expAt, err := auth.ExpToInt64(expStr)
	if err != nil {
		return err
	}

	defaultExpiryDuration, err := GetDefaultExpiration(dsecs)
	if err != nil {
		return err
	}

	// Verify if JWT expiry is lesser than default expiry duration,
	// if that is the case then set the default expiration to be
	// from the JWT expiry claim.
	if time.Unix(expAt, 0).UTC().Sub(time.Now().UTC()) < defaultExpiryDuration {
		defaultExpiryDuration = time.Unix(expAt, 0).UTC().Sub(time.Now().UTC())
	} // else honor the specified expiry duration.

	expiry := time.Now().UTC().Add(defaultExpiryDuration).Unix()
	claims["exp"] = strconv.FormatInt(expiry, 10) // update with new expiry.
	return nil
}

// Validate - validates the access token.
func (p *JWT) Validate(token, dsecs string) (map[string]interface{}, error) {
	jp := new(jwtgo.Parser)
	jp.ValidMethods = []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}

	keyFuncCallback := func(jwtToken *jwtgo.Token) (interface{}, error) {
		kid, ok := jwtToken.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("Invalid kid value %v", jwtToken.Header["kid"])
		}
		return p.publicKeys[kid], nil
	}

	var claims jwtgo.MapClaims
	jwtToken, err := jp.ParseWithClaims(token, &claims, keyFuncCallback)
	if err != nil {
		if err = p.PopulatePublicKey(); err != nil {
			return nil, err
		}
		jwtToken, err = jwtgo.ParseWithClaims(token, &claims, keyFuncCallback)
		if err != nil {
			return nil, err
		}
	}

	if !jwtToken.Valid {
		return nil, ErrTokenExpired
	}

	if err = updateClaimsExpiry(dsecs, claims); err != nil {
		return nil, err
	}

	return claims, nil

}

// ID returns the provider name and authentication type.
func (p *JWT) ID() ID {
	return "jwt"
}

// OpenID keys and envs.
const (
	JwksURL     = "jwks_url"
	ConfigURL   = "config_url"
	ClaimPrefix = "claim_prefix"
	ClientID    = "client_id"

	EnvIdentityOpenIDClientID    = "MINIO_IDENTITY_OPENID_CLIENT_ID"
	EnvIdentityOpenIDJWKSURL     = "MINIO_IDENTITY_OPENID_JWKS_URL"
	EnvIdentityOpenIDURL         = "MINIO_IDENTITY_OPENID_CONFIG_URL"
	EnvIdentityOpenIDClaimPrefix = "MINIO_IDENTITY_OPENID_CLAIM_PREFIX"
)

// DiscoveryDoc - parses the output from openid-configuration
// for example https://accounts.google.com/.well-known/openid-configuration
type DiscoveryDoc struct {
	Issuer                           string   `json:"issuer,omitempty"`
	AuthEndpoint                     string   `json:"authorization_endpoint,omitempty"`
	TokenEndpoint                    string   `json:"token_endpoint,omitempty"`
	UserInfoEndpoint                 string   `json:"userinfo_endpoint,omitempty"`
	RevocationEndpoint               string   `json:"revocation_endpoint,omitempty"`
	JwksURI                          string   `json:"jwks_uri,omitempty"`
	ResponseTypesSupported           []string `json:"response_types_supported,omitempty"`
	SubjectTypesSupported            []string `json:"subject_types_supported,omitempty"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported,omitempty"`
	ScopesSupported                  []string `json:"scopes_supported,omitempty"`
	TokenEndpointAuthMethods         []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	ClaimsSupported                  []string `json:"claims_supported,omitempty"`
	CodeChallengeMethodsSupported    []string `json:"code_challenge_methods_supported,omitempty"`
}

func parseDiscoveryDoc(u *xnet.URL, transport *http.Transport, closeRespFn func(io.ReadCloser)) (DiscoveryDoc, error) {
	d := DiscoveryDoc{}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return d, err
	}
	clnt := http.Client{
		Transport: transport,
	}
	resp, err := clnt.Do(req)
	if err != nil {
		return d, err
	}
	defer closeRespFn(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return d, err
	}
	dec := json.NewDecoder(resp.Body)
	if err = dec.Decode(&d); err != nil {
		return d, err
	}
	return d, nil
}

// DefaultKVS - default config for OpenID config
var (
	DefaultKVS = config.KVS{
		config.KV{
			Key:   ConfigURL,
			Value: "",
		},
		config.KV{
			Key:   ClientID,
			Value: "",
		},
		config.KV{
			Key:   ClaimPrefix,
			Value: "",
		},
		config.KV{
			Key:   JwksURL,
			Value: "",
		},
	}
)

// Enabled returns if jwks is enabled.
func Enabled(kvs config.KVS) bool {
	return kvs.Get(JwksURL) != ""
}

// LookupConfig lookup jwks from config, override with any ENVs.
func LookupConfig(kvs config.KVS, transport *http.Transport, closeRespFn func(io.ReadCloser)) (c Config, err error) {
	if err = config.CheckValidKeys(config.IdentityOpenIDSubSys, kvs, DefaultKVS); err != nil {
		return c, err
	}

	jwksURL := env.Get(EnvIamJwksURL, "") // Legacy
	if jwksURL == "" {
		jwksURL = env.Get(EnvIdentityOpenIDJWKSURL, kvs.Get(JwksURL))
	}

	c = Config{
		ClaimPrefix: env.Get(EnvIdentityOpenIDClaimPrefix, kvs.Get(ClaimPrefix)),
		publicKeys:  make(map[string]crypto.PublicKey),
		ClientID:    env.Get(EnvIdentityOpenIDClientID, kvs.Get(ClientID)),
		transport:   transport,
		closeRespFn: closeRespFn,
	}

	configURL := env.Get(EnvIdentityOpenIDURL, kvs.Get(ConfigURL))
	if configURL != "" {
		c.URL, err = xnet.ParseHTTPURL(configURL)
		if err != nil {
			return c, err
		}
		c.DiscoveryDoc, err = parseDiscoveryDoc(c.URL, transport, closeRespFn)
		if err != nil {
			return c, err
		}
	}
	if jwksURL == "" {
		// Fallback to discovery document jwksURL
		jwksURL = c.DiscoveryDoc.JwksURI
	}

	if jwksURL == "" {
		return c, nil
	}

	c.JWKS.URL, err = xnet.ParseHTTPURL(jwksURL)
	if err != nil {
		return c, err
	}
	if err = c.PopulatePublicKey(); err != nil {
		return c, err
	}
	return c, nil
}

// NewJWT - initialize new jwt authenticator.
func NewJWT(c Config) *JWT {
	return &JWT{c}
}
