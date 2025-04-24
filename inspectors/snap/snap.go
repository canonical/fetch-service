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

	if info, err := newSnapPackageUrlInfo(u); err == nil {
		a.SetRequestPending(ins, "valid URL for snap package").Annotate(
			Annotation{
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

// InspectArtifact extracts metadata from a known artifact file format.
func (ins *SnapInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs(mimetypes.SquashFs) { // Snaps are SquashFS filesystem images
		return nil
	}

	slog := a.Logger()

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
	snapRevisionAssertion, err := downloadSnapRevisionAssertion(snapSha3_384, slog)
	if err != nil {
		return fmt.Errorf("cannot retrieve snap-revision assertion: %w", err)
	}
	if snapRevisionAssertion.SnapSize() != fmt.Sprintf("%d", a.Size()) {
		a.SetResponseRejected(ins, "snap size mismatch in snap-revision assertion").Annotate(
			Annotation{
				"snap-revision-assertion-header": snapRevisionAssertion.Header,
			},
		)
		return nil
	}
	if snapRevisionAssertion.SnapSha384() != snapSha3_384 {
		a.SetResponseRejected(ins, "snap-revision assertion digest mismatch").Annotate(
			Annotation{
				"snap-revision-assertion-header": snapRevisionAssertion.Header,
			},
		)
		return nil
	}
	snapId := snapRevisionAssertion.SnapID()
	if snapId == "" {
		a.SetResponseRejected(ins, "cannot find snap ID in snap-revision assertion").Annotate(
			Annotation{
				"snap-revision-assertion-header": snapRevisionAssertion.Header,
			},
		)
		return nil
	}
	if err := snapRevisionAssertion.VerifySignature(slog); err != nil {
		a.SetResponseRejected(ins, "snap-revision assertion has invalid signature").Annotate(
			Annotation{
				"error-msg":                      err.Error(),
				"snap-revision-assertion-header": snapRevisionAssertion.Header,
			},
		)
		return nil
	}

	// Retrieve the snap-declaration assertion
	snapDeclarationAssertion, err := downloadSnapDeclarationAssertion(snapId, slog)
	if err != nil {
		return fmt.Errorf("cannot retrieve snap-declaration assertion: %w", err)
	}

	publisherId := snapDeclarationAssertion.PublisherID()
	if publisherId == "" {
		a.SetResponseRejected(ins, "cannot find publisher ID in snap-declaration assertion").Annotate(
			Annotation{
				"snap-declaration-assertion-header": snapDeclarationAssertion.Header,
			},
		)
		return nil
	}

	if err := snapDeclarationAssertion.VerifySignature(slog); err != nil {
		a.SetResponseRejected(ins, "snap-declaration assertion has invalid signature").Annotate(
			Annotation{
				"error-msg":                         err.Error(),
				"snap-declaration-assertion-header": snapDeclarationAssertion.Header,
			},
		)
		return nil
	}

	// Obtain the account assertion
	accountAssertion, err := downloadAccountAssertion(publisherId, slog)
	if err != nil {
		return fmt.Errorf("cannot retrieve account assertion: %w", err)
	}
	if err := accountAssertion.VerifySignature(slog); err != nil {
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
	})

	if err := checkSnapDeclarationFilter(ins.config, snapDeclarationAssertion, slog); err != nil {
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

func downloadSnapRevisionAssertionImpl(snapSha3_384 string, slog logger.Logger) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/snap-revision/%s?max-format=0", snapSha3_384)
	return downloadAssertion(url, slog)
}

var downloadSnapDeclarationAssertion = downloadSnapDeclarationAssertionImpl

func downloadSnapDeclarationAssertionImpl(snapId string, slog logger.Logger) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/snap-declaration/16/%s?max-format=5", snapId)
	return downloadAssertion(url, slog)
}

var downloadAccountAssertion = downloadAccountAssertionImpl

func downloadAccountAssertionImpl(publisherId string, slog logger.Logger) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/account/%s?max-format=5", publisherId)
	return downloadAssertion(url, slog)
}

func downloadAccountKeyAssertion(signKey string, slog logger.Logger) (*assertion, error) {
	url := fmt.Sprintf("https://api.snapcraft.io/v2/assertions/account-key/%s?max-format=1", signKey)
	return downloadAssertion(url, slog)
}

func downloadAssertion(url string, slog logger.Logger) (*assertion, error) {
	slog.Debugf("download assertion: %s", url)

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
		slog.Debugf("assertion: %+v", assert.Header)
	}

	return assert, err
}

func checkSnapDeclarationFilter(cfg config.SnapInspectorConfig, assert *assertion, slog logger.Logger) error {
	for _, v := range cfg.SnapDeclarationFilter {
		declared, ok := assert.Header[v.Name]
		slog.Debugf("snap-declaration filter: (%s, %v)", v.Name, v.Value)
		if !ok {
			// attribute not found
			return fmt.Errorf("attribute '%s' not found in the snap-declaration assertion", v.Name)
		}

		match := false
		for _, allowed := range v.Value {
			slog.Debugf("snap-declaration filter: check if %s == %s", declared, allowed)
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
