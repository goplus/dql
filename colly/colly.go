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
	"log"
	"net/http"
	"os"

	"github.com/gocolly/colly/v2"
	"github.com/goplus/dql/colly/headless"
)

const (
	XGoPackage = true
)

// -----------------------------------------------------------------------------

type App struct {
	c     *colly.Collector
	sites []*Site

	concurrency int
}

func (p *App) initApp(sites []iSiteProto) {
	p.c = colly.NewCollector()
	for _, site := range sites {
		site.initSite(p)
		site.Main()
	}
}

// CacheDir specifies a location where GET requests are cached as files.
// When it's not defined, caching is disabled.
func (p *App) CacheDir(dir string) {
	p.c.CacheDir = dir
}

func (p *App) Run() {
	c := p.c
	if c.CacheDir != "" {
		err := os.MkdirAll(c.CacheDir, os.ModePerm)
		if err != nil {
			log.Fatalln("Creating cache directory failed:", err)
		}
	}

	disp := newSiteDispatcher(p.sites)
	tr := headless.NewTransport(p.concurrency, http.DefaultTransport, func(req *http.Request) headless.RequestOptions {
		url := req.URL
		if site, ok := disp.getSite(url); ok {
			return site.reqOpts
		}
		return headless.Fallback()
	})
	c.WithTransport(tr)

	c.OnRequest(func(req *colly.Request) {
		url := req.URL
		if site, ok := disp.getSite(url); ok {
			log.Println("==> Visiting", url)
			req.Ctx.Put("site", site)
		} else {
			log.Println("==> Ignoring", url)
			req.Abort() // No site matches the URL, so abort the request.
		}
	})
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		e.Request.Visit(href)
	})
	for _, site := range p.sites {
		for _, startURL := range site.startURLs {
			c.Visit(startURL)
		}
	}
}

// -----------------------------------------------------------------------------

type iAppProto interface {
	initApp(sites []iSiteProto)
	Run()
}

type iSiteProto interface {
	initSite(theApp *App)
	Main()
}

// XGot_App_Main is required by XGo compiler as the entry of a colly project.
func XGot_App_Main(app iAppProto, sites ...iSiteProto) {
	app.initApp(sites)
	if me, ok := app.(interface{ MainEntry() }); ok {
		me.MainEntry()
	}
	app.Run()
}

// -----------------------------------------------------------------------------
