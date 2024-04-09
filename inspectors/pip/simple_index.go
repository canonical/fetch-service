// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023 Canonical Ltd.
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
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
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

func (ins *SimpleIndexInspector) InspectRequest(a *metadata.Artefact) error {
	m := indexRequestURL.FindStringSubmatch(a.CurrentDownload.URL)
	if len(m) > 1 {
		a.SetRequestOpinion(ins.ID(), opinions.Pending, "request matches valid URL").Annotate(
			metadata.Annotation{
				"match":        indexRequestURL,
				"package-name": m[1],
			},
		)
		return nil
	}

	return nil // we don't recognize this request
}

func (ins *SimpleIndexInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	pkgName, ok := a.RequestStringAnnotation(ins, "package-name")
	if !ok {
		return nil
	}

	content_type := a.CurrentDownload.ContentType

	switch {
	case a.MimeType.Is("text/html"):
		return parseHtmlIndex(ins, f, a, pkgName)
	case a.MimeType.Is("application/json") && content_type == "application/vnd.pypi.simple.v1+json":
		return parseJsonIndex(ins, f, a, pkgName)
	default:
		return nil
	}
}

func parseHtmlIndex(ins *SimpleIndexInspector, f ReadAtSeeker, a *metadata.Artefact, pkgName string) error {
	md := &a.Metadata

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
				if ok && ver == "1.1" {
					u, err := url.Parse(a.CurrentDownload.URL)
					if err != nil {
						return err
					}

					md.Name = fmt.Sprintf("Simple index for '%s'", pkgName)
					md.Version = md.Sha1.String()[:7]
					md.Description = fmt.Sprintf(
						"PyPI repository index HTML file for package '%s'", pkgName)

					var host = u.Host
					if vidx := strings.IndexByte(host, ':'); vidx > 0 {
						host = host[:vidx]
					}

					md.Vendor = host
					md.Author = host

					a.SetResponseOpinion(ins.ID(), opinions.Approved,
						"document contains pypi repository version").Annotate(
						metadata.Annotation{
							"format":             "HTML",
							"repository-version": ver,
						},
					)
					return nil
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

func parseJsonIndex(ins *SimpleIndexInspector, f ReadAtSeeker, a *metadata.Artefact, pkgName string) error {
	// FIXME: add better format verification, e.g. check schema

	md := &a.Metadata

	u, err := url.Parse(a.CurrentDownload.URL)
	if err != nil {
		return err
	}

	md.Name = fmt.Sprintf("JSON index for '%s'", pkgName)
	md.Version = "v1+json"
	md.Description = fmt.Sprintf(
		"PyPI repository index JSON file for package '%s'", pkgName)

	var host = u.Host
	if vidx := strings.IndexByte(host, ':'); vidx > 0 {
		host = host[:vidx]
	}

	md.Vendor = host
	md.Author = host

	a.SetResponseOpinion(ins.ID(), opinions.Approved, "content type is pip simple index").Annotate(
		metadata.Annotation{
			"format":             "JSON",
			"repository-version": "v1",
		},
	)

	return nil
}
