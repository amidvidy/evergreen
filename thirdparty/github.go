package thirdparty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/10gen-labs/slogger/v1"
	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/util"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	GithubBase          = "https://github.com"
	NumGithubRetries    = 3
	GithubSleepTimeSecs = 1
	GithubAPIBase       = "https://api.github.com"
)

type GithubUser struct {
	Active       bool   `json:"active"`
	DispName     string `json:"display-name"`
	EmailAddress string `json:"email"`
	FirstName    string `json:"first-name"`
	LastName     string `json:"last-name"`
	Name         string `json:"name"`
}

// GetGithubCommits returns a slice of GithubCommit objects from
// the given commitsURL when provided a valid oauth token
func GetGithubCommits(oauthToken, commitsURL string) (
	githubCommits []GithubCommit, header http.Header, err error) {
	resp, err := tryGithubGet(oauthToken, commitsURL)
	if resp == nil {
		errMsg := fmt.Sprintf("nil response from url ‘%v’", commitsURL)
		evergreen.Logger.Logf(slogger.ERROR, errMsg)
		return nil, nil, APIResponseError{errMsg}
	}
	defer resp.Body.Close()
	if err != nil {
		errMsg := fmt.Sprintf("error querying ‘%v’: %v", commitsURL, err)
		evergreen.Logger.Logf(slogger.ERROR, errMsg)
		return nil, nil, APIResponseError{errMsg}
	}

	header = resp.Header
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, ResponseReadError{err.Error()}
	}

	evergreen.Logger.Logf(slogger.INFO, "Github API response: %v. %v bytes",
		resp.Status, len(respBody))

	if resp.StatusCode != http.StatusOK {
		requestError := APIRequestError{}
		if err = json.Unmarshal(respBody, &requestError); err != nil {
			return nil, nil, APIRequestError{Message: string(respBody)}
		}
		return nil, nil, requestError
	}

	if err = json.Unmarshal(respBody, &githubCommits); err != nil {
		return nil, nil, APIUnmarshalError{string(respBody), err.Error()}
	}
	return
}

// GetGithubFile returns a struct that contains the contents of files within
// a repository as Base64 encoded content.
func GetGithubFile(oauthToken, fileURL string) (
	githubFile *GithubFile, err error) {
	resp, err := tryGithubGet(oauthToken, fileURL)
	if resp == nil {
		errMsg := fmt.Sprintf("nil response from url ‘%v’", fileURL)
		evergreen.Logger.Logf(slogger.ERROR, errMsg)
		return nil, APIResponseError{errMsg}
	}
	defer resp.Body.Close()

	if err != nil {
		errMsg := fmt.Sprintf("error querying ‘%v’: %v", fileURL, err)
		evergreen.Logger.Logf(slogger.ERROR, errMsg)
		return nil, APIResponseError{errMsg}
	}

	if resp.StatusCode != http.StatusOK {
		evergreen.Logger.Logf(slogger.ERROR, "Github API response: ‘%v’",
			resp.Status)
		if resp.StatusCode == http.StatusNotFound {
			return nil, FileNotFoundError{fileURL}
		}
		return nil, fmt.Errorf("github API returned status ‘%v’", resp.Status)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, ResponseReadError{err.Error()}
	}

	evergreen.Logger.Logf(slogger.INFO, "Github API response: %v. %v bytes",
		resp.Status, len(respBody))

	if resp.StatusCode != http.StatusOK {
		requestError := APIRequestError{}
		if err = json.Unmarshal(respBody, &requestError); err != nil {
			return nil, APIRequestError{Message: string(respBody)}
		}
		return nil, requestError
	}

	if err = json.Unmarshal(respBody, &githubFile); err != nil {
		return nil, APIUnmarshalError{string(respBody), err.Error()}
	}
	return
}

func GetCommitEvent(oauthToken, repoOwner, repo, githash string) (*CommitEvent,
	error) {
	commitURL := fmt.Sprintf("%v/repos/%v/%v/commits/%v",
		GithubAPIBase,
		repoOwner,
		repo,
		githash,
	)

	evergreen.Logger.Logf(slogger.ERROR, "requesting github commit from url: %v", commitURL)

	resp, err := tryGithubGet(oauthToken, commitURL)
	if resp == nil {
		errMsg := fmt.Sprintf("nil response from url ‘%v’", commitURL)
		evergreen.Logger.Logf(slogger.ERROR, errMsg)
		return nil, APIResponseError{errMsg}
	}

	defer resp.Body.Close()
	if err != nil {
		errMsg := fmt.Sprintf("error querying ‘%v’: %v", commitURL, err)
		evergreen.Logger.Logf(slogger.ERROR, errMsg)
		return nil, APIResponseError{errMsg}
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, ResponseReadError{err.Error()}
	}
	evergreen.Logger.Logf(slogger.INFO, "Github API response: %v. %v bytes",
		resp.Status, len(respBody))

	if resp.StatusCode != http.StatusOK {
		requestError := APIRequestError{}
		if err = json.Unmarshal(respBody, &requestError); err != nil {
			return nil, APIRequestError{Message: string(respBody)}
		}
		return nil, requestError
	}

	commitEvent := &CommitEvent{}
	if err = json.Unmarshal(respBody, commitEvent); err != nil {
		return nil, APIUnmarshalError{string(respBody), err.Error()}
	}
	return commitEvent, nil
}

// githubRequest performs the specified http request. If the oauth token field is empty it will not use oauth
func githubRequest(method string, url string, oauthToken string, data interface{}) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	// if there is data, add it to the body of the request
	if data != nil {
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		req.Body = ioutil.NopCloser(bytes.NewReader(jsonBytes))
	}

	// check if there is an oauth token, if there is make sure it is a valid oauthtoken
	if len(oauthToken) > 0 {
		if !strings.HasPrefix(oauthToken, "token ") {
			return nil, fmt.Errorf("Invalid oauth token given")
		}
		req.Header.Add("Authorization", oauthToken)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	client := &http.Client{}
	return client.Do(req)
}

func tryGithubGet(oauthToken, url string) (resp *http.Response, err error) {
	evergreen.Logger.Logf(slogger.INFO, "Attempting GitHub API call at ‘%v’", url)
	retriableGet := util.RetriableFunc(
		func() error {
			resp, err = githubRequest("GET", url, oauthToken, nil)
			if resp == nil {
				err = fmt.Errorf("empty response on getting %v", url)
			}
			if err != nil {
				evergreen.Logger.Logf(slogger.ERROR, "failed trying to call github GET on %v: %v", url, err)
				return util.RetriableError{err}
			}
			if resp.StatusCode == http.StatusUnauthorized {
				err = fmt.Errorf("Calling github GET on %v failed: got 'unauthorized' response", url)
				evergreen.Logger.Logf(slogger.ERROR, err.Error())
				return err
			}
			if resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("Calling github GET on %v got a bad response code: %v", url, resp.StatusCode)
			}
			// read the results
			rateMessage, _ := getGithubRateLimit(resp.Header)
			evergreen.Logger.Logf(slogger.DEBUG, "Github API repsonse: %v. %v", resp.Status, rateMessage)
			return nil
		},
	)

	retryFail, err := util.Retry(retriableGet, NumGithubRetries, GithubSleepTimeSecs*time.Second)
	if err != nil {
		// couldn't get it
		if retryFail {
			evergreen.Logger.Logf(slogger.ERROR, "Github GET on %v used up all retries.", err)
		}
		return nil, err
	}

	return
}

// tryGithubPost posts the data to the Github api endpoint with the url given
func tryGithubPost(url string, oauthToken string, data interface{}) (resp *http.Response, err error) {
	evergreen.Logger.Logf(slogger.ERROR, "Attempting GitHub API POST at ‘%v’", url)
	retriableGet := util.RetriableFunc(
		func() (retryError error) {
			resp, err = githubRequest("POST", url, oauthToken, data)
			if resp == nil {
				err = fmt.Errorf("empty response on getting %v", url)
			}
			if err != nil {
				evergreen.Logger.Logf(slogger.ERROR, "failed trying to call github POST on %v: %v", url, err)
				return util.RetriableError{err}
			}
			if resp.StatusCode == http.StatusUnauthorized {
				err = fmt.Errorf("Calling github POST on %v failed: got 'unauthorized' response", url)
				evergreen.Logger.Logf(slogger.ERROR, err.Error())
				return err
			}
			if resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("Calling github POST on %v got a bad response code: %v", url, resp.StatusCode)
			}
			// read the results
			rateMessage, loglevel := getGithubRateLimit(resp.Header)
			evergreen.Logger.Logf(loglevel, "Github API response: %v. %v", resp.Status, rateMessage)
			return nil
		},
	)

	retryFail, err := util.Retry(retriableGet, NumGithubRetries, GithubSleepTimeSecs*time.Second)
	if err != nil {
		// couldn't post it
		if retryFail {
			evergreen.Logger.Logf(slogger.ERROR, "Github POST on %v used up all retries.")
		}
		return nil, err
	}

	return
}

// GetGithubFileURL returns a URL that locates a github file given the owner,
// repo,remote path and revision
func GetGithubFileURL(owner, repo, remotePath, revision string) string {
	return fmt.Sprintf("https://api.github.com/repos/%v/%v/contents/%v?ref=%v",
		owner,
		repo,
		remotePath,
		revision,
	)
}

// NextPageLink returns the link to the next page for a given header's "Link"
// key based on http://developer.github.com/v3/#pagination
// For full details see http://tools.ietf.org/html/rfc5988
func NextGithubPageLink(header http.Header) string {
	hlink, ok := header["Link"]
	if !ok {
		return ""
	}

	for _, s := range hlink {
		ix := strings.Index(s, `; rel="next"`)
		if ix > -1 {
			t := s[:ix]
			op := strings.Index(t, "<")
			po := strings.Index(t, ">")
			u := t[op+1 : po]
			return u
		}
	}
	return ""
}

// getGithubRateLimit interprets the limit headers, and produces an increasingly
// alarmed message (for the caller to log) as we get closer and closer
func getGithubRateLimit(header http.Header) (message string,
	loglevel slogger.Level) {
	h := (map[string][]string)(header)
	limStr, okLim := h["X-Ratelimit-Limit"]
	remStr, okRem := h["X-Ratelimit-Remaining"]

	// ensure that we were able to read the rate limit header
	if !okLim || !okRem || len(limStr) == 0 || len(remStr) == 0 {
		message, loglevel = "Could not get rate limit data", slogger.WARN
		return
	}

	// parse the rate limits
	lim, limErr := strconv.ParseInt(limStr[0], 10, 0) // parse in decimal to int
	rem, remErr := strconv.ParseInt(remStr[0], 10, 0)

	// ensure we successfully parsed the rate limits
	if limErr != nil || remErr != nil {
		message, loglevel = fmt.Sprintf("Could not parse rate limit data: "+
			"limit=%q, rate=%q", limStr, okLim), slogger.WARN
		return
	}

	// We're in good shape
	if rem > int64(0.1*float32(lim)) {
		message, loglevel = fmt.Sprintf("Rate limit: %v/%v", rem, lim),
			slogger.INFO
		return
	}

	// we're running short
	if rem > 20 {
		message, loglevel = fmt.Sprintf("Rate limit significantly low: %v/%v",
			rem, lim), slogger.WARN
		return
	}

	// we're in trouble
	message, loglevel = fmt.Sprintf("Throttling required - rate limit almost "+
		"exhausted: %v/%v", rem, lim), slogger.ERROR
	return
}

// GithubAuthenticate does a POST to github with the code that it received, the ClientId, ClientSecret
// And returns the response which contains the accessToken associated with the user.
func GithubAuthenticate(code, clientId, clientSecret string) (githubResponse *GithubAuthResponse, err error) {
	accessUrl := "https://github.com/login/oauth/access_token"
	authParameters := GithubAuthParameters{
		ClientId:     clientId,
		ClientSecret: clientSecret,
		Code:         code,
	}
	resp, err := tryGithubPost(accessUrl, "", authParameters)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("could not authenticate for token %v", err.Error())
	}
	if resp == nil {
		return nil, fmt.Errorf("invalid github response")
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, ResponseReadError{err.Error()}
	}
	evergreen.Logger.Logf(slogger.DEBUG, "GitHub API response: %v. %v bytes",
		resp.Status, len(respBody))

	if err = json.Unmarshal(respBody, &githubResponse); err != nil {
		return nil, APIUnmarshalError{string(respBody), err.Error()}
	}
	return
}

// GetGithubUser does a GET from GitHub for the user, email, and organizations information and
// returns the GithubLoginUser and its associated GithubOrganizations after authentication
func GetGithubUser(token string) (githubUser *GithubLoginUser, githubOrganizations []GithubOrganization, err error) {
	userUrl := fmt.Sprintf("%v/user", GithubAPIBase)
	orgUrl := fmt.Sprintf("%v/user/orgs", GithubAPIBase)
	t := fmt.Sprintf("token %v", token)
	// get the user
	resp, err := tryGithubGet(t, userUrl)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, nil, err
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, ResponseReadError{err.Error()}
	}

	evergreen.Logger.Logf(slogger.INFO, "Github API response: %v. %v bytes",
		resp.Status, len(respBody))

	if err = json.Unmarshal(respBody, &githubUser); err != nil {
		return nil, nil, APIUnmarshalError{string(respBody), err.Error()}
	}

	// get the user's organizations
	resp, err = tryGithubGet(t, orgUrl)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, nil, fmt.Errorf("Could not get user from token: %v", err)
	}
	respBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, ResponseReadError{err.Error()}
	}

	evergreen.Logger.Logf(slogger.DEBUG, "Github API response: %v. %v bytes",
		resp.Status, len(respBody))

	if err = json.Unmarshal(respBody, &githubOrganizations); err != nil {
		return nil, nil, APIUnmarshalError{string(respBody), err.Error()}
	}
	return
}
