package seedcontent

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type LinkStatus string

const (
	LinkOK          LinkStatus = "ok"
	LinkInvalid     LinkStatus = "invalid_url"
	LinkBroken      LinkStatus = "broken"
	LinkUnavailable LinkStatus = "unavailable"
)

type LinkResult struct {
	URL        string     `json:"url"`
	Status     LinkStatus `json:"status"`
	HTTPStatus int        `json:"http_status,omitempty"`
}
type LinkChecker struct{ client *http.Client }

func NewLinkChecker(client *http.Client) *LinkChecker {
	if client == nil {
		client = &http.Client{}
	}
	copy := *client
	if copy.Timeout == 0 {
		copy.Timeout = 10 * time.Second
	}
	originalRedirect := copy.CheckRedirect
	copy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if err := validURL(request.URL.String()); err != nil {
			return err
		}
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if originalRedirect != nil {
			return originalRedirect(request, via)
		}
		return nil
	}
	return &LinkChecker{client: &copy}
}

func (checker *LinkChecker) Check(ctx context.Context, address string) LinkResult {
	result := LinkResult{URL: address, Status: LinkInvalid}
	if validURL(address) != nil {
		return result
	}
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		request, err := http.NewRequestWithContext(ctx, method, address, nil)
		if err != nil {
			return result
		}
		request.Header.Set("User-Agent", "GoInterviewTracker-LinkChecker/1")
		if method == http.MethodGet {
			request.Header.Set("Range", "bytes=0-1023")
		}
		response, err := checker.client.Do(request)
		if err != nil {
			if !errors.Is(err, errInvalidURL) {
				result.Status = LinkUnavailable
			}
			return result
		}
		response.Body.Close()
		result.HTTPStatus = response.StatusCode
		if method == http.MethodHead && (response.StatusCode == 405 || response.StatusCode == 501) {
			continue
		}
		switch {
		case response.StatusCode >= 200 && response.StatusCode < 300:
			result.Status = LinkOK
		case response.StatusCode >= 500 || response.StatusCode == 401 || response.StatusCode == 403 || response.StatusCode == 408 || response.StatusCode == 429:
			result.Status = LinkUnavailable
		default:
			result.Status = LinkBroken
		}
		return result
	}
	return result
}
