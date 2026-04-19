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

package snap

import (
	"errors"
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
	"github.com/canonical/fetch-service/inspectors/snap/config"
	"github.com/canonical/fetch-service/logger"
)

func SquashFsDetector(raw []byte, limit uint32) bool {
	if limit < 4 {
		return false
	}
	return slices.Compare(raw[:4], []byte("hsqs")) == 0
}

type SnapInspector struct {
	config config.SnapInspectorConfig
}

func NewSnapInspector(cfg config.SnapInspectorConfig) *SnapInspector {
	return &SnapInspector{cfg}
}

func (SnapInspector) ID() string {
	return "snap"
}

// InspectRequest verifies if the request complies with policy.
func (ins *SnapInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if info, err := newSnapPackageURLInfo(u); err == nil {
		a.SetRequestPending(ins, "valid URL for snap package").Annotate(
			Annotation{
				"snap-id": info.snapID,
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

// InspectArtifact extracts metadata from a known artifact file format.
func (ins *SnapInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs(mimetypes.SquashFs) { // Snaps are SquashFS filesystem images
		return nil
	}

	sl := a.Logger()

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
	// It's cheaper to check the assertion existence first than look for snap
	// metadata inside a potentially large squashfs file that's not a snap.
	snapRevisionAssertion, err := downloadSnapRevisionAssertion(snapSha3_384, sl)
	if err != nil {
		sl.Debugf("cannot retrieve snap-revision assertion: %s", err)
		return nil // This is (probably) not a snap
	}

	snapID, note, err := checkSnapRevisionAssertion(snapRevisionAssertion, snapSha3_384, sl, a)
	if err != nil {
		a.SetResponseRejected(ins, err.Error()).Annotate(note)
		return nil
	}

	// Retrieve the snap-declaration assertion
	snapDeclarationAssertion, err := downloadSnapDeclarationAssertion(snapID, sl)
	if err != nil {
		return fmt.Errorf("cannot retrieve snap-declaration assertion: %w", err)
	}

	publisherID := snapDeclarationAssertion.PublisherID()
	if publisherID == "" {
		a.SetResponseRejected(ins, "cannot find publisher ID in snap-declaration assertion").Annotate(
			Annotation{
				"snap-declaration-assertion-header": snapDeclarationAssertion.Header,
			},
		)
		return nil
	}

	if err := snapDeclarationAssertion.VerifySignature(sl); err != nil {
		a.SetResponseRejected(ins, "snap-declaration assertion has invalid signature").Annotate(
			Annotation{
				"error-msg":                         err.Error(),
				"snap-declaration-assertion-header": snapDeclarationAssertion.Header,
			},
		)
		return nil
	}

	// Obtain the account assertion
	accountAssertion, err := downloadAccountAssertion(publisherID, sl)
	if err != nil {
		return fmt.Errorf("cannot retrieve account assertion: %w", err)
	}
	if err := accountAssertion.VerifySignature(sl); err != nil {
		a.SetResponseRejected(ins, "account assertion has invalid signature").Annotate(
			Annotation{
				"error-msg":                err.Error(),
				"account-assertion-header": accountAssertion.Header,
			},
		)
		return nil
	}

	// Extract additional metadata from snap.yaml
	sqsh, err := squashfs.NewReader(f)
	if err != nil {
		return err
	}
	sf, err := sqsh.Open("meta/snap.yaml")
	if err != nil {
		a.SetResponseRejected(ins, "image has no meta/snap.yaml file")
		return nil // it's not a snap package
	}
	defer sf.Close()

	var data snapYaml
	dec := yaml.NewDecoder(sf)
	if err := dec.Decode(&data); err != nil {
		a.SetResponseRejected(ins, "cannot decode meta/snap.yaml")
		return nil
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:          mimetypes.SnapPackage,
		Name:          snapDeclarationAssertion.SnapName(),
		Version:       data.Version,
		Description:   data.Summary,
		License:       data.License,
		Vendor:        accountAssertion.DisplayName(),
		Architecture:  strings.Join(data.Architectures, ","),
		StoreRevision: snapRevisionAssertion.SnapRevision(),
		ContentID:     snapID,
	})

	if err := checkSnapDeclarationFilter(ins.config, snapDeclarationAssertion, sl); err != nil {
		a.SetResponseRejected(ins, "failure on snap-declaration assertion attribute check").Annotate(
			Annotation{
				"error-msg":                         err.Error(),
				"snap-declaration-assertion-header": snapDeclarationAssertion.Header,
			},
		)
		return nil
	}

	a.SetResponseApproved(ins, "valid snap file found").Annotate(
		Annotation{
			"snap-revision-assertion-header":    snapRevisionAssertion.Header,
			"snap-declaration-assertion-header": snapDeclarationAssertion.Header,
			"account-assertion-header":          accountAssertion.Header,
		},
	)

	return nil
}

var downloadSnapRevisionAssertion = downloadSnapRevisionAssertionImpl

func downloadSnapRevisionAssertionImpl(snapSha3_384 string, sl logger.Logger) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/snap-revision/%s?max-format=0", snapSha3_384)
	return downloadAssertion(url, sl)
}

var downloadSnapDeclarationAssertion = downloadSnapDeclarationAssertionImpl

func downloadSnapDeclarationAssertionImpl(snapID string, sl logger.Logger) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/snap-declaration/16/%s?max-format=5", snapID)
	return downloadAssertion(url, sl)
}

var downloadAccountAssertion = downloadAccountAssertionImpl

func downloadAccountAssertionImpl(publisherID string, sl logger.Logger) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/account/%s?max-format=5", publisherID)
	return downloadAssertion(url, sl)
}

func downloadAccountKeyAssertion(signKey string, sl logger.Logger) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/account-key/%s?max-format=1", signKey)
	return downloadAssertion(url, sl)
}

func downloadAssertion(url string, sl logger.Logger) (*assertion, error) {
	sl.Debugf("download assertion: %s", url)

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

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot download assertion: %s", res.Status)
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	assert, err := newAssertion(data)
	if err == nil {
		sl.Debugf("assertion: %+v", assert.Header)
	}

	return assert, err
}

func checkSnapDeclarationFilter(cfg config.SnapInspectorConfig, assert *assertion, sl logger.Logger) error {
	for _, v := range cfg.SnapDeclarationFilter {
		declared, ok := assert.Header[v.Name]
		sl.Debugf("snap-declaration filter: (%s, %v)", v.Name, v.Value)
		if !ok {
			// attribute not found
			return fmt.Errorf("attribute '%s' not found in the snap-declaration assertion", v.Name)
		}

		match := false
		for _, allowed := range v.Value {
			sl.Debugf("snap-declaration filter: check if %s == %s", declared, allowed)
			if declared == allowed {
				match = true
				break
			}
		}

		if !match {
			// attribute found but value does not match
			return fmt.Errorf("attribute '%s' value '%s' is not allowed", v.Name, declared)
		}
	}

	return nil
}

// checkSnapRevisionAssertion checks the snap revision assertion and extracts the snap ID
func checkSnapRevisionAssertion(snapRevisionAssertion *assertion, snapSha3_384 string, sl logger.Logger, a ResponseArtifact) (string, Annotation, error) {
	if snapRevisionAssertion.SnapSize() != fmt.Sprintf("%d", a.Size()) {
		return "", Annotation{
			"snap-revision-assertion-header": snapRevisionAssertion.Header,
		}, errors.New("snap size mismatch in snap-revision assertion")
	}
	if snapRevisionAssertion.SnapSha384() != snapSha3_384 {
		return "", Annotation{
			"snap-revision-assertion-header": snapRevisionAssertion.Header,
		}, errors.New("snap-revision assertion digest mismatch")
	}
	snapID := snapRevisionAssertion.SnapID()
	if snapID == "" {
		return "", Annotation{
			"snap-revision-assertion-header": snapRevisionAssertion.Header,
		}, errors.New("cannot find snap ID in snap-revision assertion")
	}
	if err := snapRevisionAssertion.VerifySignature(sl); err != nil {
		return "", Annotation{
			"error-msg":                      err.Error(),
			"snap-revision-assertion-header": snapRevisionAssertion.Header,
		}, errors.New("snap-revision assertion has invalid signature")
	}
	return snapID, nil, nil
}
