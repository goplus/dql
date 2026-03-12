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

package colly

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/goplus/dql/colly/headless"
)

const (
	defaultTimeout       = 30 * time.Second
	defaultJSExecTimeout = 5 * time.Second
)

// -----------------------------------------------------------------------------

// Headless specifies the headless browser settings of the site.
type Headless struct {
	WaitReady     any
	Timeout       time.Duration
	JSExecTimeout time.Duration
}

func (p *Headless) render(url string) (html string, err error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	timeout := p.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	jsexecTimeout := p.JSExecTimeout
	if jsexecTimeout == 0 {
		jsexecTimeout = defaultJSExecTimeout
	}

	waitReady := p.WaitReady
	if waitReady == nil {
		waitReady = "body"
	}

	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady(waitReady),
		chromedp.Sleep(jsexecTimeout),
		chromedp.OuterHTML("html", &html),
	)
	return
}

// -----------------------------------------------------------------------------

// Site represents a website to be crawled.
type Site struct {
	baseURLs  []string
	startURLs []string
	reqOpts   headless.RequestOptions
}

func (p *Site) initSite(app *App) {
	app.sites = append(app.sites, p)
}

func (p *Site) getBaseURLs() []*url.URL {
	if len(p.baseURLs) > 0 {
		urls := make([]*url.URL, len(p.baseURLs))
		for i, baseURL := range p.baseURLs {
			u, e := url.Parse(baseURL)
			if e != nil {
				log.Fatalln("Parsing base URL failed:", e)
			}
			if !strings.HasSuffix(u.Path, "/") {
				u.Path += "/"
			}
			urls[i] = u
		}
		return urls
	}
	if len(p.startURLs) > 0 {
		u, e := url.Parse(p.startURLs[0])
		if e != nil {
			log.Fatalln("Parsing start URL failed:", e)
		}
		return []*url.URL{{
			Scheme: u.Scheme,
			Host:   u.Host,
			Path:   "/",
		}}
	}
	return nil
}

// BaseURL specifies the base URLs of the site. Call it when a site has multiple
// base URLs for caching effectively.
func (p *Site) BaseURL(baseURLs ...string) {
	p.baseURLs = baseURLs
}

// Start specifies the start URLs of the site for crawling.
func (p *Site) Start(startURLs ...string) {
	p.startURLs = startURLs
}

// HeadlessWaitVisible specifies that the headless browser should wait until the
// element matching the selector is visible before rendering the page.
func (p *Site) HeadlessWaitVisible(selector string) {
	p.reqOpts = headless.WaitVisible(selector)
}

// HeadlessWaitReady specifies that the headless browser should wait until the
// element matching the JavaScript path is ready before rendering the page.
func (p *Site) HeadlessWaitReady(jsPath string) {
	p.reqOpts = headless.WaitReady(jsPath)
}

// HeadlessWaitFunc specifies a custom function that the headless browser should
// execute before rendering the page.
func (p *Site) HeadlessWaitFunc(fn func(ctx context.Context) error) {
	p.reqOpts = headless.WaitFunc(fn)
}

// -----------------------------------------------------------------------------

type host struct {
	scheme, host string
}

type pathDispatcher struct {
	base string
	site *Site
}

type siteDispatcher struct {
	hosts map[host][]pathDispatcher
}

func newSiteDispatcher(sites []*Site) *siteDispatcher {
	hosts := make(map[host][]pathDispatcher)
	for _, site := range sites {
		for _, baseURL := range site.getBaseURLs() {
			host := host{scheme: baseURL.Scheme, host: baseURL.Host}
			hosts[host] = append(hosts[host], pathDispatcher{
				base: baseURL.Path,
				site: site,
			})
		}
	}
	return &siteDispatcher{hosts: hosts}
}

func (p siteDispatcher) getSite(u *url.URL) (*Site, bool) {
	host := host{scheme: u.Scheme, host: u.Host}
	for _, pathDisp := range p.hosts[host] {
		if strings.HasPrefix(u.Path, pathDisp.base) {
			return pathDisp.site, true
		}
	}
	return nil, false
}

// -----------------------------------------------------------------------------
