package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"github.com/dgrijalva/jwt-go"
)

type ParsedTokenResponse struct {
	AccessToken AccessTokenClaims
	ExpiresIn int64
	Jti string
	RefreshToken string
	Scope string
	TokenType string
}

type AccessTokenClaims struct {
	Aud []string `json:"aud"`
	Scope []string `json:"scope"`
	Username string `json:"user_name"`
	Email string `json:"email"`
	jwt.StandardClaims
}

func assertInt64(t *testing.T, a int64, b int64) {
	if (a != b) {
		t.Errorf("Expected '%d' == '%d'", a, b);
	}
}

func assertString(t *testing.T, a string, b string) {
	if (a != b) {
		t.Errorf("Expected '%s' == '%s'", a, b);
	}
}

func assertStatus(t *testing.T, recorder *httptest.ResponseRecorder, code int) {
	if recorder.Code != code {
		t.Errorf("Expected code %d, got %d", code, recorder.Code)
	}
}

func assertHeader(t *testing.T, recorder *httptest.ResponseRecorder, header string, value string) {
	actualValue := recorder.HeaderMap.Get(header)
	if actualValue != value {
		t.Errorf("Expected header '%s' to be '%s', but it is '%s'",
			header, value, actualValue)
	}
}

func assertBody(t *testing.T, recorder *httptest.ResponseRecorder, value string) {
	actualValue := recorder.Body.String()
	if actualValue != value {
		t.Errorf("Expected body '%s', got '%s'", value, actualValue)
	}
}

func assertBodyMatches(t *testing.T, recorder *httptest.ResponseRecorder, restr string) {
	actualValue := recorder.Body.String()
	re := regexp.MustCompile(restr)
	if !re.MatchString(actualValue) {
		t.Errorf("Expected body '%s' to match '%s'", actualValue, restr)
	}
}

func assertError(t *testing.T, err error, message string) {
	if (err.Error() != message) {
		t.Errorf("Expected error '%s' to be '%s'", err.Error(), message)
	}
}

func handle(request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()

	handler, err := NewServerHandler(&ServerConfig{
		CallbackUrl: Urlify("http://client/callback"),
		AccessTokenLifetime: 600,
	})

	if (err != nil)  {
		panic(err.Error())
	}

	handler(recorder, request)

	return recorder
}

func TestNewServerHandlerReturnsErrWhenConfigIsNil(t *testing.T) {
	_, err := NewServerHandler(nil)

	assertError(t, err, "config must be non-nil")
}

func TestNewServerHandlerReturnsErrWhenConfigCallbackUrlIsNil(t *testing.T) {
	_, err := NewServerHandler(&ServerConfig{
		CallbackUrl: nil,
	})

	assertError(t, err, "config.CallbackUrl must be non-nil")
}

func TestLoginPageWorksWithoutQueryArgs(t *testing.T) {
	recorder := handle(&http.Request{
		Method: "GET",
		URL:    Urlify("/oauth/authorize"),
	})

	assertStatus(t, recorder, 200)
	assertHeader(t, recorder, "Content-Type", "text/html")
}

func TestLoginPageWorksWithQueryArgs(t *testing.T) {
	recorder := handle(&http.Request{
		Method: "GET",
		URL:    Urlify("/oauth/authorize?state=blah"),
	})

	assertStatus(t, recorder, 200)
	assertHeader(t, recorder, "Content-Type", "text/html")
	assertBodyMatches(t, recorder, `type="hidden" name="state" value="blah"`)
}

func TestRedirectToCallbackWorks(t *testing.T) {
	recorder := handle(&http.Request{
		Method: "GET",
		URL:    Urlify("/oauth/authorize?email=foo&state=bar"),
	})

	assertStatus(t, recorder, 302)
	assertHeader(t, recorder, "Location",
		"http://client/callback?code=foo&state=bar")
}

func assertTokenError(t *testing.T, postForm url.Values, body string) {
	recorder := handle(&http.Request{
		Method:   "POST",
		URL:      Urlify("/oauth/token"),
		PostForm: postForm,
	})

	assertStatus(t, recorder, 400)
	assertHeader(t, recorder, "Content-Type", "text/plain")
	assertBody(t, recorder, body)
}

func TestTokenErrorsWhenCodeIsEmpty(t *testing.T) {
	assertTokenError(t, url.Values{
		"client_id": []string{"boop"},
		"client_secret": []string{"bap"},
		"grant_type": []string{"authorization_code"},
	}, "'code' is missing or empty")
}

func TestTokenErrorsWhenGrantTypeIsInvalid(t *testing.T) {
	assertTokenError(t, url.Values{
		"client_id": []string{"boop"},
		"client_secret": []string{"bap"},
		"grant_type": []string{"wut"},
	}, "'grant_type' must be 'authorization_code' or 'refresh_token'")
}

func TestRefreshAccessTokenErrorsWhenRefreshTokenIsMissing(t *testing.T) {
	assertTokenError(t, url.Values{
		"client_id": []string{"boop"},
		"client_secret": []string{"bap"},
		"grant_type": []string{"refresh_token"},
	}, "'refresh_token' is missing or malformed")	
}

func TestRefreshAccessTokenErrorsWhenRefreshTokenIsMalformed(t *testing.T) {
	assertTokenError(t, url.Values{
		"client_id": []string{"boop"},
		"client_secret": []string{"bap"},
		"grant_type": []string{"refresh_token"},
		"refresh_token": []string{"blarg:foo@bar.com"},
	}, "'refresh_token' is missing or malformed")	
}

func GetTokenResponse(t *testing.T, postForm url.Values, response *ParsedTokenResponse) {
	recorder := handle(&http.Request{
		Method: "POST",
		URL:    Urlify("/oauth/token"),
		PostForm: postForm,
	})

	assertStatus(t, recorder, 200)
	assertHeader(t, recorder, "Content-Type", "application/json")

	var rawResponse tokenResponse

	err := json.Unmarshal(recorder.Body.Bytes(), &rawResponse)

	if err != nil {
		t.Errorf("Error unmarshaling response: %s", err.Error())
	}

	response.ExpiresIn = rawResponse.ExpiresIn
	response.Jti = rawResponse.Jti
	response.RefreshToken = rawResponse.RefreshToken
	response.Scope = rawResponse.Scope
	response.TokenType = rawResponse.TokenType

	token, err := jwt.ParseWithClaims(rawResponse.AccessToken, &AccessTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte("unused secret key (for verification)"), nil
	})

	if err != nil {
		t.Errorf("Error parsing access token JWT: %s", err.Error())
	}

	if !token.Valid {
		t.Errorf("Access token JWT is invalid")
	}

	claims, ok := token.Claims.(*AccessTokenClaims)

	if !ok {
		t.Errorf("Claims are not OK")
	}

	response.AccessToken = *claims
}

func TestRefreshAccessTokenWorks(t *testing.T) {
	var response ParsedTokenResponse

	GetTokenResponse(t, url.Values{
		"client_id":     []string{"baz"},
		"client_secret": []string{"baz"},
		"grant_type":    []string{"refresh_token"},
		"refresh_token": []string{"fake_oauth2_refresh_token:foo@bar.com"},
	}, &response)

	assertString(t, response.RefreshToken, "fake_oauth2_refresh_token:foo@bar.com")
	assertString(t, response.AccessToken.Username, "foo@bar.com")
	assertString(t, response.AccessToken.Email, "foo@bar.com")
}

func TestExchangeCodeForAccessTokenErrorsWhenClientIdIsEmpty(t *testing.T) {
	assertTokenError(t, url.Values{
		"code": []string{"foo@bar.gov"},
	}, "'client_id' is missing or empty")
}

func TestExchangeCodeForAccessTokenWorks(t *testing.T) {
	var response ParsedTokenResponse

	GetTokenResponse(t, url.Values{
		"code":          []string{"foo@bar.gov"},
		"client_id":     []string{"baz"},
		"client_secret": []string{"baz"},
		"grant_type":    []string{"authorization_code"},
		"response_type": []string{"token"},
	}, &response)

	assertString(t, response.RefreshToken, "fake_oauth2_refresh_token:foo@bar.gov")
	assertInt64(t, response.ExpiresIn, 10 * 60)
	assertString(t, response.AccessToken.Username, "foo@bar.gov")
	assertString(t, response.AccessToken.Email, "foo@bar.gov")
}

func TestGetSvgLogoWorks(t *testing.T) {
	recorder := handle(&http.Request{
		Method: "GET",
		URL:    Urlify("/fake-cloud.gov.svg"),
	})

	assertStatus(t, recorder, 200)
	assertHeader(t, recorder, "Content-Type", "image/svg+xml")
}

func TestGetStylesheetWorks(t *testing.T) {
	recorder := handle(&http.Request{
		Method: "GET",
		URL:    Urlify("/style.css"),
	})

	assertStatus(t, recorder, 200)
	assertHeader(t, recorder, "Content-Type", "text/css")
}

func Test404Works(t *testing.T) {
	recorder := handle(&http.Request{
		Method: "GET",
		URL:    Urlify("/blah"),
	})

	assertStatus(t, recorder, 404)
	assertHeader(t, recorder, "Content-Type", "text/plain")
}
