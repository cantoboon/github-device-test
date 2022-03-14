package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

var (
	client http.Client
)

const (
	clientId = "a5db2e56355d5992c59f"
	debug    = true
)

func main() {
	client = http.Client{}

	deviceCode := GetDeviceCode()

	expiration := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	fmt.Printf("Please open the following link in your browser: %s\n", deviceCode.VerificationUri)

	fmt.Printf("\nWhen prompted, enter the following code: %s\n", deviceCode.UserCode)

	fmt.Println("\nWe will poll for the authorization code every", deviceCode.Interval, "seconds until", expiration.Format("15:04:05"))

	time.Sleep(time.Duration(deviceCode.Interval) * time.Second)

	for time.Now().Before(expiration) {
		token := PollAccessToken(clientId, deviceCode.DeviceCode)

		if token.AccessToken != "" {
			userData := HandleAccessToken(*token)
			fmt.Println("\nHello", userData.Name)
			os.Exit(0)
		} else if token.Error != "" {
			switch token.Error {
			case AuthorizationPending:
				time.Sleep(time.Duration(deviceCode.Interval) * time.Second)
				continue
			case SlowDown:
				if debug {
					log.Println("Request was too quick, slowing down")
				}
				time.Sleep(time.Duration(token.Interval) * time.Second)
			case ExpiredToken:
				if debug {
					log.Fatalln("Token has expired", token.ErrorDescription)
				}
			case UnsupportedGrantType:
				if debug {
					log.Fatalln("Unsupported grant type", token.ErrorDescription)
				}
			case IncorrectClientCredentials:
				if debug {
					log.Fatalln("This shouldn't happen for device flow, seems to be a thing for the web app flow", token.ErrorDescription)
				}
			case AccessDenied:
				log.Fatalln("Access denied: Did you click cancel on the authorization page?")
			}
		} else {
			log.Fatalln("Not sure how we would reach this point. No AccessToken or Error?")
		}
	}
	log.Fatalln("The authorization request has timed out")
}

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationUri string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func GetDeviceCode() DeviceCodeResponse {
	request, err := http.NewRequest("POST", "https://github.com/login/device/code", bytes.NewBufferString(url.Values{"client_id": {clientId}}.Encode()))
	if err != nil {
		log.Fatalln(err)
	}

	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		log.Fatalln(err)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalln("failed to parse body", err)
	}

	if debug {
		log.Println("Device code body: ", string(body))
	}

	var deviceCode DeviceCodeResponse
	err = json.Unmarshal(body, &deviceCode)
	if err != nil {
		log.Fatalln("failed to unmarshall device code", err)
	}
	return deviceCode
}

// AccessTokenResponse comes from the https://github.com/login/oauth/access_token endpoint
// You'll either get an AccessToken, TokenType, and Scope OR Error, ErrorDescription, ErrorUri and optionally an Interval
// when the Error is slow_down
type AccessTokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	ErrorUri         string `json:"error_uri"`
	Interval         int    `json:"interval "`
}

// https://docs.github.com/en/developers/apps/building-oauth-apps/authorizing-oauth-apps#error-codes-for-the-device-flow
const (
	AuthorizationPending       = "authorization_pending"
	SlowDown                   = "slow_down"
	ExpiredToken               = "expired_token"
	UnsupportedGrantType       = "unsupported_grant_type"
	IncorrectClientCredentials = "incorrect_client_credentials"
	AccessDenied               = "access_denied"
)

// PollAccessToken makes a single POST request to the https://github.com/login/oauth/access_token endpoint to see
// if the authorization process has been completed. The AccessTokenResponse will contain either the AccessToken or
// the error. Errors are expected until authorization has been successfully completed
func PollAccessToken(clientId string, deviceCode string) *AccessTokenResponse {
	values := url.Values{
		"client_id":   {clientId},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	request, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewBufferString(values.Encode()))
	if err != nil {
		fmt.Println("failed to create access token request", err)
		return nil
	}
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		fmt.Println("failed to make request", err)
		return nil
	}

	if response.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Println("failed to read body", err)
			return nil
		}
		var accessToken AccessTokenResponse
		if debug {
			log.Println("Access Token response body: ", string(body))
		}
		err = json.Unmarshal(body, &accessToken)
		if err != nil {
			log.Println("failed to unmarshall json", err)
			return nil
		}
		return &accessToken
	} else {
		body, err := ioutil.ReadAll(response.Body)
		log.Fatalln("Received status code: ", response.StatusCode, ". This is unexpected. We've never seen this before. Body: ", string(body), " - Error: ", err)
	}
	return nil
}

// UserDataResponse contains all the data we're interested in from the https://api.github.com/user endpoint
type UserDataResponse struct {
	Name string `json:"name"`
}

// HandleAccessToken takes the Access Token to show off what it can access
func HandleAccessToken(token AccessTokenResponse) *UserDataResponse {
	request, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		log.Fatalln("failed to create request", err)
	}

	request.Header.Set("Authorization", fmt.Sprintf("%s %s", token.TokenType, token.AccessToken))

	response, err := client.Do(request)
	if err != nil {
		return nil
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println("failed to read body", err)
	}
	if debug {
		log.Println("Access Token response body: ", string(body))
	}
	var userData UserDataResponse
	err = json.Unmarshal(body, &userData)
	if err != nil {
		log.Fatalln("failed to parse user data", err)
	}
	return &userData
}
