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
	"os"
	"time"

	"github.com/opencontainers/image-spec/specs-go"
	spec "github.com/opencontainers/image-spec/specs-go/v1"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/oci"
	"github.com/canonical/fetch-service/metadata"
	"github.com/opencontainers/go-digest"
)

func (s *ociSuite) TestRegistryIsAllowed(c *C) {
	cfg := getTestOciConfig()

	name, ok := oci.RegistryIsAllowed(registryUrl, &cfg)
	c.Assert(ok, Equals, true)
	c.Assert(name, Equals, "default")

	name, ok = oci.RegistryIsAllowed("https://foo.bar", &cfg)
	c.Assert(ok, Equals, false)
	c.Assert(name, Equals, "")
}

func (s *ociSuite) TestCheckArtifactDigest(c *C) {
	d := "4c033dfdd85cb24a108d416a9ebbdcace7faca46cedc3ab22778203375b1afbf"
	expected := digest.Digest("sha256:" + d)

	f, err := files.OpenArtifactFile("testdata/hello/blobs/sha256/" + d)
	c.Assert(err, IsNil)

	a := metadata.NewArtifact()
	a.Metadata.Sha256 = mustSha256(d)

	err = oci.CheckArtifactDigest(expected, f, a)
	c.Assert(err, IsNil)
}

func (s *ociSuite) TestParseOciImageIndex(c *C) {
	f, err := os.Open("testdata/hello/index.json")
	c.Assert(err, IsNil)

	index, err := oci.ParseOciImageIndex(f)
	c.Assert(err, IsNil)
	c.Assert(index, DeepEquals, &spec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: "application/vnd.oci.image.index.v1+json",
		Manifests: []spec.Descriptor{{
			MediaType: "application/vnd.oci.image.manifest.v1+json",
			Digest:    digest.Digest("sha256:4c033dfdd85cb24a108d416a9ebbdcace7faca46cedc3ab22778203375b1afbf"),
			Size:      400,
			Annotations: map[string]string{
				"io.containerd.image.name":          "docker.io/library/hello:3.0",
				"org.opencontainers.image.ref.name": "3.0",
			},
		}},
	})
}

func (s *ociSuite) TestParseOciImageManifest(c *C) {
	f, err := os.Open("testdata/hello/blobs/sha256/4c033dfdd85cb24a108d416a9ebbdcace7faca46cedc3ab22778203375b1afbf")
	c.Assert(err, IsNil)

	mfest, err := oci.ParseOciImageManifest(f)
	c.Assert(err, IsNil)
	c.Assert(mfest, DeepEquals, &spec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Config: spec.Descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    digest.Digest("sha256:f225928cec07b8af72ce2cafad237c72a112ab8ab77a5d435a611a60b89d6abf"),
			Size:      733,
		},
		Layers: []spec.Descriptor{{
			MediaType: "application/vnd.oci.image.layer.v1.tar",
			Digest:    digest.Digest("sha256:daffd85f19b69f25973221fe049c6d2302e4001ff0b7b35f97803208cb5c7c82"),
			Size:      2396672,
		}},
	})
}

func (s *ociSuite) TestParseOciImageConfig(c *C) {
	f, err := os.Open("testdata/hello/blobs/sha256/f225928cec07b8af72ce2cafad237c72a112ab8ab77a5d435a611a60b89d6abf")
	c.Assert(err, IsNil)

	created, err := time.Parse(time.RFC3339Nano, "2025-04-24T19:21:20.254103676+06:00")
	c.Assert(err, IsNil)

	cfg, err := oci.ParseOciImageConfig(f)
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, &spec.Image{
		Created: &created,
		Platform: spec.Platform{
			Architecture: "amd64",
			OS:           "linux",
		},
		Config: spec.ImageConfig{
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			},
			Entrypoint: []string{"/usr/bin/hello"},
			WorkingDir: "/",
		},
		RootFS: spec.RootFS{
			Type: "layers",
			DiffIDs: []digest.Digest{
				"sha256:daffd85f19b69f25973221fe049c6d2302e4001ff0b7b35f97803208cb5c7c82",
			},
		},
		History: []spec.History{{
			Created:   &created,
			CreatedBy: "COPY --parents /lib* /usr/lib/*-linux-*/ld-linux*.so* /usr/lib64/ld-linux*.so* /usr/lib/*-linux-*/libc.so* /usr/bin/hello / # buildkit",
			Comment:   "buildkit.dockerfile.v0",
		}, {
			Created:    &created,
			CreatedBy:  `ENTRYPOINT ["/usr/bin/hello"]`,
			Comment:    "buildkit.dockerfile.v0",
			EmptyLayer: true,
		}},
	})
}
