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
	"os"

	"github.com/gocolly/colly/v2"
)

const (
	XGoPackage = true
)

// -----------------------------------------------------------------------------

type App struct {
	c     *colly.Collector
	sites []*Site
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
		os.MkdirAll(c.CacheDir, os.ModePerm)
	}
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		e.Request.Visit(href)
	})
	c.OnRequest(func(r *colly.Request) {
		log.Println("==> Visiting", r.URL.String())
	})
	for _, site := range p.sites {
		for _, startURL := range site.startURLs {
			c.Visit(startURL)
		}
	}
}

// -----------------------------------------------------------------------------

type Site struct {
	c         *colly.Collector
	baseURLs  []string
	startURLs []string
}

func (p *Site) initSite(app *App) {
	p.c = app.c
	app.sites = append(app.sites, p)
}

func (p *Site) BaseURL(baseURLs ...string) {
	p.baseURLs = baseURLs
}

func (p *Site) Start(startURLs ...string) {
	p.startURLs = startURLs
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
