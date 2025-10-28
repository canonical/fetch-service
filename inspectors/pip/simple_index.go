// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package pip

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	. "github.com/canonical/fetch-service/inspectors/common"
)

var (
	indexRequestURL = regexp.MustCompile(`^https://pypi.org:443/simple/([\w-]+)/$`)
)

type SimpleIndexInspector struct {
}

func NewSimpleIndexInspector() *SimpleIndexInspector {
	return &SimpleIndexInspector{}
}

func (SimpleIndexInspector) ID() string {
	return "pip.simple-index"
}

func (ins *SimpleIndexInspector) InspectRequest(a RequestArtifact) error {
	m := indexRequestURL.FindStringSubmatch(a.DownloadURL())
	if len(m) > 1 {
		// Request marked as Unknown because it comes from the default pypi origin
		a.SetRequestUnknown(ins, "unsupported origin").Annotate(
			Annotation{
				"match":        indexRequestURL,
				"package-name": m[1],
			},
		)
		return nil
	}

	return nil // we don't recognize this request
}

func (ins *SimpleIndexInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	pkgName, ok := a.RequestStringAnnotation(ins.ID(), "package-name")
	if !ok {
		return nil
	}

	contentType := a.ContentType()

	switch {
	case a.MimetypeIs("text/html"):
		return parseHtmlIndex(ins, f, a, pkgName)
	case a.MimetypeIs("application/json") && contentType == "application/vnd.pypi.simple.v1+json":
		return parseJsonIndex(ins, f, a, pkgName)
	default:
		return nil
	}
}

func parseHtmlIndex(ins *SimpleIndexInspector, f ArtifactReader, a ResponseArtifact, pkgName string) error {
	z := html.NewTokenizer(f)

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if err := z.Err(); err != io.EOF {
				return err
			}
			return nil // end of file

		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if t.Data == "meta" {
				ver, ok := extractMetaProperty(t, "pypi:repository-version")
				if !ok {
					break
				}
				if ver == "1.1" || ver == "1.2" {
					u, err := url.Parse(a.DownloadURL())
					if err != nil {
						return err
					}

					var host = u.Host
					if vidx := strings.IndexByte(host, ':'); vidx > 0 {
						host = host[:vidx]
					}

					a.SetArtifactMetadata(ArtifactMetadata{
						Type:        "text/html",
						Name:        fmt.Sprintf("Simple index for '%s'", pkgName),
						Description: fmt.Sprintf("PyPI repository index HTML file for package '%s'", pkgName),
						Vendor:      host,
						Author:      host,
					})

					a.SetResponseApproved(ins, "document contains pypi repository version").Annotate(
						Annotation{
							"format":             "HTML",
							"repository-version": ver,
						},
					)
					return nil
				} else {
					a.SetResponseRejected(ins, "unknown pypi repository version").Annotate(
						Annotation{
							"format":             "HTML",
							"repository-version": ver,
						},
					)

				}
			}
		}
	}
}

func extractMetaProperty(t html.Token, name string) (content string, ok bool) {
	for _, attr := range t.Attr {
		if attr.Key == "name" && attr.Val == name {
			ok = true
		}

		if attr.Key == "content" {
			content = attr.Val
		}
	}

	return
}

func parseJsonIndex(ins *SimpleIndexInspector, f ArtifactReader, a ResponseArtifact, pkgName string) error {
	// FIXME: add better format verification, e.g. check schema

	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return err
	}

	var host = u.Host
	if vidx := strings.IndexByte(host, ':'); vidx > 0 {
		host = host[:vidx]
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        "application/json",
		Name:        fmt.Sprintf("JSON index for '%s'", pkgName),
		Version:     "v1+json",
		Description: fmt.Sprintf("PyPI repository index JSON file for package '%s'", pkgName),
		Vendor:      host,
		Author:      host,
	})

	a.SetResponseApproved(ins, "content type is pip simple index").Annotate(
		Annotation{
			"format":             "JSON",
			"repository-version": "v1",
		},
	)

	return nil
}
