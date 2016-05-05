// Copyright 2012 Jimmy Zelinskie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2016 Samir Bhatt. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package geddit implements an abstraction for the reddit.com API.
package geddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/google/go-querystring/query"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/clientcredentials"
)

// AppOnlyOAuthSession represents an OAuth session with reddit.com --
// all authenticated API calls are methods bound to this type.
type AppOnlyOAuthSession struct {
	Client       *http.Client
	ClientID     string
	ClientSecret string
	OAuthConfig  *clientcredentials.Config
	TokenExpiry  time.Time
	UserAgent    string
	ctx          context.Context
	Debug        bool
}

// NewAppOnlyOAuthSession creates a new session for those who want to log into a
// reddit account via Application Only OAuth.
// See https://github.com/reddit/reddit/wiki/OAuth2#application-only-oauth
func NewAppOnlyOAuthSession(clientID, clientSecret, useragent string, debug bool) (*AppOnlyOAuthSession, error) {
	s := &AppOnlyOAuthSession{}

	if useragent != "" {
		s.UserAgent = useragent
	} else {
		s.UserAgent = "Geddit API Client https://github.com/imheresamir/geddit"
	}

	s.ClientID = clientID
	s.ClientSecret = clientSecret

	// Set OAuth config
	s.OAuthConfig = &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     "https://www.reddit.com/api/v1/access_token",
	}

	s.ctx = context.Background()

	return s, nil
}

// refreshToken should be called internally before each API call
func (a *AppOnlyOAuthSession) refreshToken() error {
	// Check if token needs to be refreshed
	if time.Now().Before(a.TokenExpiry) {
		return nil
	}

	// Fetch OAuth token
	t, err := a.OAuthConfig.Token(a.ctx)
	if err != nil {
		return err
	}
	a.TokenExpiry = t.Expiry

	a.Client = a.OAuthConfig.Client(a.ctx)
	return nil
}

func (a *AppOnlyOAuthSession) getBody(link string, d interface{}) error {
	a.refreshToken()

	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return err
	}

	// This is needed to avoid rate limits
	req.Header.Set("User-Agent", a.UserAgent)

	if a.Client == nil {
		return errors.New("OAuth Session lacks HTTP client! Error getting token")
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	// DEBUG
	if a.Debug {
		fmt.Printf("***DEBUG***\nRequest Body: %s\n***DEBUG***\n\n", body)
	}

	err = json.Unmarshal(body, d)
	if err != nil {
		return err
	}

	return nil
}

// Listing returns a slice of Submission pointers.
// See https://www.reddit.com/dev/api#listings for documentation.
func (a *AppOnlyOAuthSession) Listing(username, listing string, sort popularitySort, params ListingOptions) ([]*Submission, error) {
	p, err := query.Values(params)
	if err != nil {
		return nil, err
	}
	if sort != "" {
		p.Set("sort", string(sort))
	}

	type resp struct {
		Data struct {
			Children []struct {
				Data *Submission
			}
		}
	}
	r := &resp{}
	url := fmt.Sprintf("https://oauth.reddit.com/user/%s/%s?%s", username, listing, p.Encode())
	err = a.getBody(url, r)
	if err != nil {
		return nil, err
	}

	submissions := make([]*Submission, len(r.Data.Children))
	for i, child := range r.Data.Children {
		submissions[i] = child.Data
	}

	return submissions, nil
}

func (a *AppOnlyOAuthSession) Upvoted(username string, sort popularitySort, params ListingOptions) ([]*Submission, error) {
	return a.Listing(username, "upvoted", sort, params)
}

// AboutRedditor returns a Redditor for the given username using OAuth.
func (a *AppOnlyOAuthSession) AboutRedditor(user string) (*Redditor, error) {
	type redditor struct {
		Data Redditor
	}
	r := &redditor{}
	link := fmt.Sprintf("https://oauth.reddit.com/user/%s/about", user)

	err := a.getBody(link, r)
	if err != nil {
		return nil, err
	}
	return &r.Data, nil
}

func (a *AppOnlyOAuthSession) UserTrophies(user string) ([]*Trophy, error) {
	type trophyData struct {
		Data struct {
			Trophies []struct {
				Data Trophy
			}
		}
	}

	t := &trophyData{}
	url := fmt.Sprintf("https://oauth.reddit.com/api/v1/user/%s/trophies", user)
	err := a.getBody(url, t)
	if err != nil {
		return nil, err
	}

	var trophies []*Trophy
	for _, trophy := range t.Data.Trophies {
		trophies = append(trophies, &trophy.Data)
	}
	return trophies, nil
}

// AboutSubreddit returns a subreddit for the given subreddit name using OAuth.
func (a *AppOnlyOAuthSession) AboutSubreddit(name string) (*Subreddit, error) {
	type subreddit struct {
		Data Subreddit
	}
	sr := &subreddit{}
	link := fmt.Sprintf("https://oauth.reddit.com/r/%s/about", name)

	err := a.getBody(link, sr)
	if err != nil {
		return nil, err
	}
	return &sr.Data, nil
}

// Comments returns the comments for a given Submission using OAuth.
func (a *AppOnlyOAuthSession) Comments(h *Submission, sort popularitySort, params ListingOptions) ([]*Comment, error) {
	p, err := query.Values(params)
	if err != nil {
		return nil, err
	}
	var c interface{}
	link := fmt.Sprintf("https://oauth.reddit.com/comments/%s?%s", h.ID, p.Encode())
	err = a.getBody(link, &c)
	if err != nil {
		return nil, err
	}
	helper := new(helper)
	helper.buildComments(c)
	return helper.comments, nil
}

// SubredditSubmissions returns the submissions on the given subreddit using OAuth.
func (a *AppOnlyOAuthSession) SubredditSubmissions(subreddit string, sort popularitySort, params ListingOptions) ([]*Submission, error) {
	v, err := query.Values(params)
	if err != nil {
		return nil, err
	}

	baseUrl := "https://oauth.reddit.com"

	// If subbreddit given, add to URL
	if subreddit != "" {
		baseUrl += "/r/" + subreddit
	}

	redditURL := fmt.Sprintf(baseUrl+"/%s.json?%s", sort, v.Encode())

	type Response struct {
		Data struct {
			Children []struct {
				Data *Submission
			}
		}
	}

	r := new(Response)
	err = a.getBody(redditURL, r)
	if err != nil {
		return nil, err
	}

	submissions := make([]*Submission, len(r.Data.Children))
	for i, child := range r.Data.Children {
		submissions[i] = child.Data
	}

	return submissions, nil
}

// Frontpage returns the submissions on the default reddit frontpage using OAuth.
func (a *AppOnlyOAuthSession) Frontpage(sort popularitySort, params ListingOptions) ([]*Submission, error) {
	return a.SubredditSubmissions("", sort, params)
}
