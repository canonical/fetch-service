// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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

package oci_test

import (
	"testing"

	spec "github.com/opencontainers/image-spec/specs-go/v1"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/glob"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/oci"
	"github.com/canonical/fetch-service/inspectors/oci/config"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/gabriel-vasile/mimetype"
)

type ociSuite struct{}

func (t *ociSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&ociSuite{})

func Test(t *testing.T) { TestingT(t) }

const (
	registryUrl     = "https://oci.registry"
	registryAuthUrl = "https://oci.registry/auth/"
)

func getTestOciConfig() config.OciInspectorConfig {
	return config.OciInspectorConfig{
		Registries: map[string]config.OciInspectorConfigRegistry{
			"default": {
				Url:     glob.MustCompile(registryUrl),
				AuthUrl: glob.MustCompile(registryAuthUrl + "*"),
			},
		},
	}
}

func (s *ociSuite) TestOciInspectorID(c *C) {
	ins := oci.NewOciInspector(getTestOciConfig())
	c.Assert(ins.ID(), Equals, "oci")
}

type ociInspectTest struct {
	summary string                   // Test summary.
	reqs    []ociInspectCompleteTest // Requests and artifacts.
}

type ociInspectCompleteTest struct {
	summary  string                  // Request summary.
	request  *ociInspectRequestTest  // Request. Must not be nil.
	artifact *ociInspectArtifactTest // Artifact. If nil, skip artifact inspection.
}

type ociInspectRequestTest struct {
	url  string               // Request URL.
	err  string               // Expected request inspection error.
	op   opinions.OpinionKind // Expected request inspection opinion.
	rsn  string               // Expected request inspection opinion reason.
	note Annotation           // Expected request inspection annotation.
}

type ociInspectArtifactTest struct {
	file string               // Artifact file path.
	dgst string               // Artifact digest, SHA256.
	mime string               // Artifact mime type.
	err  string               // Expected artifact inspection error.
	op   opinions.OpinionKind // Expected artifact inspection opinion.
	rsn  string               // Expected artifact inspection opinion reason.
	meta metadata.Metadata    // Expected artifact metadata.
	note Annotation           // Expected artifact annotation.
}

var ociInspectorTests = []ociInspectTest{
	idealImagePull,
}

func (s *ociSuite) TestOciInspector(c *C) {
	for _, tc := range ociInspectorTests {
		c.Logf("Summary: %s", tc.summary)

		cfg := getTestOciConfig()
		ins := oci.NewOciInspector(cfg)

		for _, r := range tc.reqs {
			c.Logf("Request summary: %s", r.summary)
			c.Assert(r, NotNil)

			// Prepare request artifact.
			a := metadata.NewArtifact()
			a.CurrentDownload.URL = r.request.url

			// Inspect request.
			err := ins.InspectRequest(a)
			if r.request.err != "" {
				c.Assert(err, ErrorMatches, r.request.err)
				continue
			}
			c.Assert(err, IsNil)

			// Check request opinion, reason and annotation.
			reqInsp, ok := a.RequestInspection[ins.ID()]
			if r.request.op == opinions.Pending || r.request.op == opinions.Rejected || ok {
				c.Assert(ok, Equals, true)
				c.Assert(reqInsp.Opinion, Equals, r.request.op)
				c.Assert(reqInsp.Reason, Equals, r.request.rsn)
				c.Assert(reqInsp.Annotations, DeepEquals, r.request.note)
			}

			if r.artifact == nil {
				continue
			}

			// Prepare response artifact.
			a.MimeType = mimetype.Lookup(r.artifact.mime)
			a.Metadata.Type = r.artifact.mime
			a.Metadata.Sha256, err = digests.NewSha256Digest(r.artifact.dgst)
			c.Assert(err, IsNil)

			f, err := files.OpenArtifactFile(r.artifact.file)
			c.Assert(err, IsNil)
			defer f.Close()

			// Inspect artifact.
			err = ins.InspectArtifact(f, a)
			if r.artifact.err != "" {
				c.Assert(err, ErrorMatches, r.artifact.err)
				continue
			}
			c.Assert(err, IsNil)

			// Check artifact opinion, reason and annotation.
			artInsp, ok := a.ResponseInspection[ins.ID()]
			if r.artifact.op == opinions.Approved || r.artifact.op == opinions.Rejected || ok {
				c.Assert(ok, Equals, true)
				c.Assert(artInsp.Opinion, Equals, r.artifact.op)
				c.Assert(artInsp.Reason, Equals, r.artifact.rsn)
				c.Assert(artInsp.Annotations, DeepEquals, r.artifact.note)
				c.Assert(a.Metadata, DeepEquals, r.artifact.meta)
			}
		}
	}
}

func mustSha256(d string) digests.Sha256Digest {
	h, err := digests.NewSha256Digest(d)
	if err != nil {
		panic(err)
	}
	return h
}

// -- Test data --

// This test data describes an ideal image pull workflow, where the client
// should do the following network operations:
//
//  1. Ping the registry at the /v2/ endpoint to see if the registry supports
//     the OCI distribution spec.[^1]
//     This typically results in a 401 Unauthorized response with the
//     "www-authenticate" header set to the authentication URL. Since inspectors
//     do not get to see the response headers on a 401 response, the OCI
//     inspector takes in an "auth-url" configuration parameter which describes
//     the authentication URL, as "www-authenticate" would set it.
//
//  2. Make a request to the authentication URL to generate access token.
//
//  3. Pull the image index file using the following endpoint:
//     ...       /v2/<image-name>/manifests/<reference>        ("end-3" at [^1])
//     <reference> can either be a tag or digest. The client would then parse
//     the image index to get the digests for the manifests.
//
//  4. Pull the image manifest using the above ("end-3") endpoint. The client
//     would then parse the manifest file to get the digests of the config file
//     and the layers blob.
//
//  5. Pull the config and layer blobs, using the following endpoint:
//     ...       /v2/<image-name>/blobs/<digest>               ("end-2" at [^1])
//
// References:
// - [^1] https://github.com/opencontainers/distribution-spec/blob/main/spec.md#endpoints
var idealImagePull = ociInspectTest{
	summary: "An ideal image pull (hello:3.0)",
	reqs: []ociInspectCompleteTest{{
		// Popular registries such as Dockerhub, ECR sends a 401 response to
		// this ping request, with "www-authenticate" header set.
		// In this test, we assume that the future internal registry will be
		// ideal and send a 200 OK response (with no body).
		summary: "1. Ping registry at /v2/ endpoint",
		request: &ociInspectRequestTest{
			url: registryUrl + "/v2/",
			op:  opinions.Pending,
			rsn: "valid registry ping",
			note: Annotation{
				"request-type":  "ping",
				"registry-name": "default",
				"registry-url":  registryUrl,
			},
		},
		artifact: &ociInspectArtifactTest{
			file: "testdata/empty-file",
			dgst: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			op:   opinions.Approved,
			rsn:  "valid registry ping artifact",
			meta: metadata.Metadata{
				Sha256: mustSha256("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"),
			},
		},
	}, {
		// Authentication requests typically sends a 200 OK response with a JSON
		// payload which contains an access token.
		summary: "2. Authentication request",
		request: &ociInspectRequestTest{
			url: registryAuthUrl + "?service=registry&scope=oci",
			op:  opinions.Pending,
			rsn: "valid registry auth request",
			note: Annotation{
				"request-type":  "auth",
				"registry-name": "default",
			},
		},
		artifact: &ociInspectArtifactTest{
			file: "testdata/auth-response.json",
			dgst: "cc42785204be763053e0fb45abfb37e1414529ae211bd126547b3c336c15e43f",
			mime: "application/json",
			op:   opinions.Approved,
			rsn:  "valid registry authentication artifact",
			meta: metadata.Metadata{
				Type:   "application/json",
				Sha256: mustSha256("cc42785204be763053e0fb45abfb37e1414529ae211bd126547b3c336c15e43f"),
			},
		},
	}, {
		summary: "3. Get image index",
		request: &ociInspectRequestTest{
			url: registryUrl + "/v2/hello/manifests/3.0",
			op:  opinions.Pending,
			rsn: "valid manifest pull request",
			note: Annotation{
				"request-type":  "manifest",
				"registry-name": "default",
				"registry-url":  registryUrl,
				"image-name":    "hello",
				"reference":     "3.0",
			},
		},
		artifact: &ociInspectArtifactTest{
			file: "testdata/hello/index.json",
			dgst: "91fb68afb192bf93da988faa6a4c086168ccab5aedfbc94072dca90e37c631ec",
			mime: spec.MediaTypeImageIndex,
			op:   opinions.Approved,
			rsn:  "valid OCI image index",
			meta: metadata.Metadata{
				Type:        spec.MediaTypeImageIndex,
				Sha256:      mustSha256("91fb68afb192bf93da988faa6a4c086168ccab5aedfbc94072dca90e37c631ec"),
				Name:        "hello",
				Version:     "3.0",
				Description: "hello:3.0 image index",
			},
			note: Annotation{},
		},
	}, {
		summary: "4. Get image manifests",
		request: &ociInspectRequestTest{
			url: registryUrl + "/v2/hello/manifests/sha256:4c033dfdd85cb24a108d416a9ebbdcace7faca46cedc3ab22778203375b1afbf",
			op:  opinions.Pending,
			rsn: "valid manifest pull request",
			note: Annotation{
				"request-type":  "manifest",
				"registry-name": "default",
				"registry-url":  registryUrl,
				"image-name":    "hello",
				"reference":     "sha256:4c033dfdd85cb24a108d416a9ebbdcace7faca46cedc3ab22778203375b1afbf",
				"digest":        "sha256:4c033dfdd85cb24a108d416a9ebbdcace7faca46cedc3ab22778203375b1afbf",
			},
		},
		artifact: &ociInspectArtifactTest{
			file: "testdata/hello/blobs/sha256/4c033dfdd85cb24a108d416a9ebbdcace7faca46cedc3ab22778203375b1afbf",
			dgst: "4c033dfdd85cb24a108d416a9ebbdcace7faca46cedc3ab22778203375b1afbf",
			mime: spec.MediaTypeImageManifest,
			op:   opinions.Approved,
			rsn:  "valid OCI image manifest",
			meta: metadata.Metadata{
				Type:        spec.MediaTypeImageManifest,
				Sha256:      mustSha256("4c033dfdd85cb24a108d416a9ebbdcace7faca46cedc3ab22778203375b1afbf"),
				Name:        "hello",
				Version:     "sha256:4c033dfdd85cb24a108d416a9ebbdcace7faca46cedc3ab22778203375b1afbf",
				Description: "hello:sha256:4c033dfdd85cb24a108d416a9ebbdcace7faca46cedc3ab22778203375b1afbf image manifest",
			},
			note: Annotation{
				"io.containerd.image.name":          "docker.io/library/hello:3.0",
				"org.opencontainers.image.ref.name": "3.0",
			},
		},
	}, {
		summary: "5a. Get image blobs - config",
		request: &ociInspectRequestTest{
			url: registryUrl + "/v2/hello/blobs/sha256:f225928cec07b8af72ce2cafad237c72a112ab8ab77a5d435a611a60b89d6abf",
			op:  opinions.Pending,
			rsn: "valid blob pull request",
			note: Annotation{
				"request-type":  "blob",
				"registry-name": "default",
				"registry-url":  registryUrl,
				"image-name":    "hello",
				"reference":     "sha256:f225928cec07b8af72ce2cafad237c72a112ab8ab77a5d435a611a60b89d6abf",
				"digest":        "sha256:f225928cec07b8af72ce2cafad237c72a112ab8ab77a5d435a611a60b89d6abf",
			},
		},
		artifact: &ociInspectArtifactTest{
			file: "testdata/hello/blobs/sha256/f225928cec07b8af72ce2cafad237c72a112ab8ab77a5d435a611a60b89d6abf",
			dgst: "f225928cec07b8af72ce2cafad237c72a112ab8ab77a5d435a611a60b89d6abf",
			mime: "application/json",
			op:   opinions.Approved,
			rsn:  "valid OCI image blob",
			meta: metadata.Metadata{
				Type:        spec.MediaTypeImageConfig,
				Sha256:      mustSha256("f225928cec07b8af72ce2cafad237c72a112ab8ab77a5d435a611a60b89d6abf"),
				Name:        "hello",
				Version:     "sha256:f225928cec07b8af72ce2cafad237c72a112ab8ab77a5d435a611a60b89d6abf",
				Description: "hello:sha256:f225928cec07b8af72ce2cafad237c72a112ab8ab77a5d435a611a60b89d6abf image blob",
			},
			note: Annotation{},
		},
	}, {
		summary: "5b. Get image blobs - layer",
		request: &ociInspectRequestTest{
			url: registryUrl + "/v2/hello/blobs/sha256:daffd85f19b69f25973221fe049c6d2302e4001ff0b7b35f97803208cb5c7c82",
			op:  opinions.Pending,
			rsn: "valid blob pull request",
			note: Annotation{
				"request-type":  "blob",
				"registry-name": "default",
				"registry-url":  registryUrl,
				"image-name":    "hello",
				"reference":     "sha256:daffd85f19b69f25973221fe049c6d2302e4001ff0b7b35f97803208cb5c7c82",
				"digest":        "sha256:daffd85f19b69f25973221fe049c6d2302e4001ff0b7b35f97803208cb5c7c82",
			},
		},
		artifact: &ociInspectArtifactTest{
			file: "testdata/hello/blobs/sha256/daffd85f19b69f25973221fe049c6d2302e4001ff0b7b35f97803208cb5c7c82",
			dgst: "daffd85f19b69f25973221fe049c6d2302e4001ff0b7b35f97803208cb5c7c82",
			op:   opinions.Approved,
			rsn:  "valid OCI image blob",
			meta: metadata.Metadata{
				Type:        spec.MediaTypeImageLayer,
				Sha256:      mustSha256("daffd85f19b69f25973221fe049c6d2302e4001ff0b7b35f97803208cb5c7c82"),
				Name:        "hello",
				Version:     "sha256:daffd85f19b69f25973221fe049c6d2302e4001ff0b7b35f97803208cb5c7c82",
				Description: "hello:sha256:daffd85f19b69f25973221fe049c6d2302e4001ff0b7b35f97803208cb5c7c82 image blob",
			},
			note: Annotation{},
		},
	}},
}
