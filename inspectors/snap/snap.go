// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024 Canonical Ltd.
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

package snap

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/CalebQ42/squashfs"
	"gopkg.in/yaml.v3"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
)

func SquashFsDetector(raw []byte, limit uint32) bool {
	return slices.Compare(raw[:4], []byte("hsqs")) == 0
}

type SnapInspector struct {
}

func NewSnapInspector() *SnapInspector {
	return &SnapInspector{}
}

func (SnapInspector) ID() string {
	return "snap"
}

// InspectRequest verifies if the request complies with policy.
func (ins SnapInspector) InspectRequest(a *metadata.Artefact) error {
	u, err := url.Parse(a.CurrentDownload.URL)
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if info, err := newSnapPackageUrlInfo(u); err == nil {
		a.Hold(ins, "valid URL for snap package").Annotate(
			metadata.Annotation{
				"snap-id": info.snapId,
				"release": info.release,
			},
		)
	}

	return nil // we don't recognize this request
}

type snapYaml struct {
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Summary       string   `json:"summary"`
	License       string   `json:"license,omitempty"`
	Architectures []string `json:"architectures"`
	Grade         string   `json:"grade"`
	Base          string   `json:"base"`
}

// InspectArtefact extracts metadata from a known artefact file format.
func (ins *SnapInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	if a.Metadata.Type != mimetypes.SquashFs { // Snaps are SquashFS filesystem images
		return nil
	}

	digest, err := computeDigest(f)
	if err != nil {
		return fmt.Errorf("cannot compute digest: %w", err)
	}

	snapSha3_384, err := encodeDigest(digest)
	if err != nil {
		return fmt.Errorf("cannot encode digest: %w", err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Retrieve snap-revision assertion
	snapRevisionAssertion, err := downloadSnapRevisionAssertion(snapSha3_384)
	if err != nil {
		return fmt.Errorf("cannot retrieve snap-revision assertion: %w", err)
	}
	if err := snapRevisionAssertion.VerifySignature(); err != nil {
		a.Reject(ins, "snap-revision assertion has invalid signature").Annotate(
			metadata.Annotation{
				"error-msg":                      err.Error(),
				"snap-revision-assertion-header": snapRevisionAssertion.Header,
			},
		)
		return nil
	}
	if snapRevisionAssertion.SnapSha384() != snapSha3_384 {
		a.Reject(ins, "snap-revision assertion digest mismatch").Annotate(
			metadata.Annotation{
				"snap-revision-assertion-header": snapRevisionAssertion.Header,
			},
		)
		return nil
	}
	if snapRevisionAssertion.SnapSize() != fmt.Sprintf("%d", a.Metadata.Size) {
		a.Reject(ins, "snap-revision assertion size mismatch").Annotate(
			metadata.Annotation{
				"snap-revision-assertion-header": snapRevisionAssertion.Header,
			},
		)
		return nil
	}

	snapId := snapRevisionAssertion.SnapID()
	if snapId == "" {
		a.Reject(ins, "cannot find snap-id in snap-revision assertion").Annotate(
			metadata.Annotation{
				"snap-revision-assertion-header": snapRevisionAssertion.Header,
			},
		)
		return nil
	}

	// Retrieve the snap-declaration assertion
	snapDeclarationAssertion, err := downloadSnapDeclarationAssertion(snapId)
	if err != nil {
		return fmt.Errorf("cannot retrieve snap-declaration assertion: %w", err)
	}
	if err := snapDeclarationAssertion.VerifySignature(); err != nil {
		a.Reject(ins, "snap-declaration assertion has invalid signature").Annotate(
			metadata.Annotation{
				"error-msg":                         err.Error(),
				"snap-declaration-assertion-header": snapDeclarationAssertion.Header,
			},
		)
		return nil
	}

	publisherId := snapDeclarationAssertion.PublisherID()
	if publisherId == "" {
		a.Reject(ins, "cannot find publisher-id in snap-declaration assertion").Annotate(
			metadata.Annotation{
				"snap-declaration-assertion-header": snapDeclarationAssertion.Header,
			},
		)
		return nil
	}

	// Obtain the account assertion
	accountAssertion, err := downloadAccountAssertion(publisherId)
	if err != nil {
		return fmt.Errorf("cannot retrieve account assertion: %w", err)
	}
	if err := accountAssertion.VerifySignature(); err != nil {
		a.Reject(ins, "account assertion has invalid signature").Annotate(
			metadata.Annotation{
				"error-msg":                err.Error(),
				"account-assertion-header": accountAssertion.Header,
			},
		)
		return nil
	}

	// Extract additional metadata from snap.yaml
	sqsh, err := squashfs.NewReader(f)
	if err != nil {
		return nil
	}
	sf, err := sqsh.Open("meta/snap.yaml")
	if err != nil {
		a.Comment(ins, "image has no meta/snap.yaml file")
		return nil // it's not a snap package
	}
	defer sf.Close()

	var data snapYaml
	dec := yaml.NewDecoder(sf)
	if err := dec.Decode(&data); err != nil {
		a.Reject(ins, "cannot decode meta/snap.yaml")
		return nil
	}

	a.Metadata.Type = mimetypes.SnapPackage

	a.Metadata.Name = snapDeclarationAssertion.SnapName()
	a.Metadata.Version = snapRevisionAssertion.SnapRevision()
	a.Metadata.Description = data.Summary
	a.Metadata.License = data.License
	a.Metadata.Architecture = strings.Join(data.Architectures, ",")
	a.Metadata.Vendor = accountAssertion.DisplayName()

	a.Approve(ins, "valid snap file found").Annotate(
		metadata.Annotation{
			"snap-revision-assertion-header":    snapRevisionAssertion.Header,
			"snap-declaration-assertion-header": snapDeclarationAssertion.Header,
			"account-assertion-header":          accountAssertion.Header,
		},
	)

	return nil
}

func downloadSnapRevisionAssertion(snapSha3_384 string) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/snap-revision/%s?max-format=0", snapSha3_384)
	return downloadAssertion(url)
}

func downloadSnapDeclarationAssertion(snapId string) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/snap-declaration/16/%s?max-format=5", snapId)
	return downloadAssertion(url)
}

func downloadAccountAssertion(publisherId string) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/account/%s?max-format=5", publisherId)
	return downloadAssertion(url)
}

func downloadAccountKeyAssertion(signKey string) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/account-key/%s?max-format=1", signKey)
	return downloadAssertion(url)
}

func downloadAssertion(url string) (*assertion, error) {
	logger.Debugf("download assertion: %s", url)

	client := http.Client{
		Transport: &http.Transport{},
		Timeout:   60 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/x.ubuntu.assertion")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	assert, err := newAssertion(data)
	if err == nil {
		logger.Debugf("assertion: %+v", assert.Header)
	}

	return assert, err
}
