# GitHub Device Flow Authentication

A single file Golang application that performs GitHub's [Device Flow authentication](https://docs.github.com/en/developers/apps/building-oauth-apps/authorizing-oauth-apps#device-flow).

The Device Flow uses the [OAuth 2.0 Device Authorization Grant workflow](https://tools.ietf.org/html/rfc8628).
You may have seen this style of workflow on TV based applications like Netflix, Disney+, etc. The
TV will show you a code and either a link or QR code that takes you to a page where you login and then type in the code.

## [1. App requests the device and user verification codes from GitHub](https://docs.github.com/en/developers/apps/building-oauth-apps/authorizing-oauth-apps#step-1-app-requests-the-device-and-user-verification-codes-from-github)

We send a `POST https://github.com/login/device/code` request using a Client ID which is given when setting up a
[GitHub OAuth application](https://github.com/settings/applications/new).

```json
{"device_code":"c5c540e811259858f9a9348db20da9b185dde7ce","user_code":"B7PD-AB09","verification_uri":"https://github.com/login/device","expires_in":899,"interval":5}
```

## 2. Prompt the user to enter the user code in a browser

We print the link to the login page and the user code for them to type in.

```
Please open the following link in your browser: https://github.com/login/device

When prompted, enter the following code: B7BD-AB09

We will poll for the authorization code every 5 seconds until 14:15:16
```

The user then needs to open the page and enter the user code. 

## 3. App polls GitHub to check if the user authorized the device

In the meantime, the application will poll the access token endpoint.
The documentation for this step is a little light. The following should be useful to know.

1. The `POST https://github.com/login/oauth/access_token` endpoint will always return status 200.
2. The content returned will differ based on the state above.

When the request is still pending (ie. user hasn't entered the user code), you will see this:

```json
{"error":"authorization_pending","error_description":"The authorization request is still pending.","error_uri":"https://docs.github.com/developers/apps/authorizing-oauth-apps#error-codes-for-the-device-flow"}
```

If you poll too frequently, you will receive this:

```json
{"error":"slow_down","error_description":"Too many requests have been made in the same timeframe.","error_uri":"https://docs.github.com","interval":10}
```

Once authorization has been granted:

```json
{"access_token":"gho_DuMHYwjYigM1Ggkhq84N2iWbqmfknp1uQhTE","token_type":"bearer","scope":""}
```

## Bonus: Use the access token

Now we can use the `access_token` with the GitHub API.

```go
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
```