/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package headless

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// RequestOptions defines options for individual requests that can be used to
// customize the headless rendering behavior on a per-request basis.
type RequestOptions struct {
	waitBeforeFetch chromedp.QueryAction
	fallback        bool
}

// Fallback returns a RequestOptions that indicates the request should bypass
// headless rendering and be handled by the fallback transport instead.
func Fallback() RequestOptions {
	return RequestOptions{fallback: true}
}

// WaitVisible returns a RequestOptions that waits for the specified element query
// to be visible before fetching the rendered HTML. This is useful for pages that
// load content dynamically and you want to ensure the content is present before
// extraction. The selector selects a single element by the DOM.querySelector
// command.
func WaitVisible(selector string) RequestOptions {
	return RequestOptions{
		waitBeforeFetch: chromedp.WaitVisible(selector, chromedp.ByQuery),
	}
}

// WaitReady returns a RequestOptions that waits for the specified element query
// to be present in the DOM before fetching the rendered HTML. This is a more
// lenient wait condition than WaitVisible, as it does not require the element to
// be visible. The selector is interpreted as a JavaScript path expression that
// selects an element from the global window object, e.g. "document.body".
func WaitReady(jsPath string) RequestOptions {
	return RequestOptions{
		waitBeforeFetch: chromedp.WaitReady(jsPath, chromedp.ByJSPath),
	}
}

// WaitFunc returns a RequestOptions that waits for the specified custom function
// to return nil before fetching the rendered HTML. This allows for arbitrary
// conditions to be implemented using the full power of the chromedp API. The
// function receives a chromedp context and should perform any necessary actions
// (e.g. waiting for multiple elements, checking JavaScript variables, etc.) and
// return nil when the page is ready to be fetched.
func WaitFunc(fn func(ctx context.Context) error) RequestOptions {
	return RequestOptions{
		waitBeforeFetch: chromedp.ActionFunc(fn),
	}
}

// Transport implements http.RoundTripper.
// It intercepts HTTP GET requests, renders them with a headless Chrome instance
// via chromedp, and returns the fully-rendered HTML as the response body.
type Transport struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc

	timeout   time.Duration
	semaphore chan struct{}

	reqOpts  func(req *http.Request) RequestOptions
	fallback http.RoundTripper

	once   sync.Once
	closed bool
}

// NewTransport creates a Transport and starts the shared Chrome allocator.
// Call Close() when done to release the browser process.
//
// `concurrency` limits the number of browser tabs open simultaneously.
//
// `callback` is the http.RoundTripper to use for requests that should bypass
// headless rendering.
//
// `reqOpts` is called for each GET request to determine whether to use
// headless rendering or fall back to the fallback transport.
func NewTransport(concurrency int, callback http.RoundTripper, reqOpts func(req *http.Request) RequestOptions) *Transport {
	t := &Transport{
		timeout:   30 * time.Second,
		semaphore: make(chan struct{}, concurrency),
		reqOpts:   reqOpts,
		fallback:  callback,
	}

	allocOpts := append(slices.Clone(chromedp.DefaultExecAllocatorOptions[:]),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true), // TODO(xsw): check this
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
	)
	t.allocCtx, t.allocCancel = chromedp.NewExecAllocator(
		context.Background(), allocOpts...,
	)
	return t
}

// SetTimeout sets the per-page render timeout.
func (t *Transport) SetTimeout(d time.Duration) {
	t.timeout = d
}

// RoundTrip implements http.RoundTripper.
// Non-GET requests are forwarded to the fallback transport (or rejected if none is set).
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.closed {
		return nil, fmt.Errorf("headless transport: already closed")
	}

	var opts RequestOptions
	if req.Method == http.MethodGet {
		opts = t.reqOpts(req)
	} else {
		opts.fallback = true
	}

	// Only intercept GET requests that don't match the fallback criteria.
	if opts.fallback {
		return t.fallback.RoundTrip(req)
	}

	// Acquire a concurrency slot; blocks when the tab limit is reached.
	t.semaphore <- struct{}{}
	defer func() { <-t.semaphore }()

	html, status, finalURL, err := t.render(req, opts.waitBeforeFetch)
	if err != nil {
		return t.fallback.RoundTrip(req)
	}
	return buildResponse(req, html, status, finalURL), nil
}

// render launches a new browser tab, navigates to the request URL,
// waits for the page to be ready, and returns the outer HTML.
func (t *Transport) render(req *http.Request, waitBeforeFetch chromedp.Action) (html string, status int, finalURL string, err error) {
	// Each tab gets its own context derived from the shared allocator.
	tabCtx, tabCancel := chromedp.NewContext(t.allocCtx)
	defer tabCancel()

	ctx, cancel := context.WithTimeout(tabCtx, t.timeout)
	defer cancel()

	status = 200
	finalURL = req.URL.String()
	err = chromedp.Run(
		ctx, chromedp.Navigate(finalURL),
		waitBeforeFetch,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Location(&finalURL).Do(ctx)
		}),
		chromedp.OuterHTML("html", &html),
	)
	return
}

// buildResponse wraps the rendered HTML string into a well-formed *http.Response
// that satisfies the contract expected by http.Client and colly.
func buildResponse(req *http.Request, html string, statusCode int, finalURL string) *http.Response {
	body := io.NopCloser(strings.NewReader(html))
	resp := &http.Response{
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		StatusCode: statusCode,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       body,
		Request:    req,
	}

	resp.Header.Set("Content-Type", "text/html; charset=utf-8")
	resp.Header.Set("X-Final-URL", finalURL) // actual URL after any redirects
	resp.Header.Set("X-Rendered-By", "chromedp")

	return resp
}

// Close shuts down the shared Chrome process.
// It is safe to call multiple times; subsequent calls are no-ops.
func (t *Transport) Close() {
	t.once.Do(func() {
		t.closed = true
		t.allocCancel()
	})
}
