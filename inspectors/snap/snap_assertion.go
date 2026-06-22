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
	"net/url"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

func AssertionDetector(raw []byte, limit uint32) bool {
	if len(raw) < 128 {
		return false
	}

	lines := strings.Split(string(raw[:128]), "\n")
	if len(lines) < 3 {
		return false
	}

	hasAuthID := false
	for i := range lines {
		if strings.HasPrefix(lines[i], "authority-id:") {
			hasAuthID = true
			break
		}
	}

	return strings.HasPrefix(lines[0], "type:") && hasAuthID
}

// SnapAssertionInspector examines assertion requests and files.
//
// Assertions are text-based and take a context-dependent format
// that always includes one or more headers, an optional body, and
// the encoded signature.
type SnapAssertionInspector struct {
}

func NewSnapAssertionInspector() *SnapAssertionInspector {
	return &SnapAssertionInspector{}
}

func (SnapAssertionInspector) ID() string {
	return "snap.assertion"
}

// InspectRequest verifies if the request complies with policy.
func (ins *SnapAssertionInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	// There are different types of assertion. Here we're interested
	// in snap-revision, snap-declaration, account, and account-key
	// assertion types.
	//
	// The snap-revision type stores acknowledgement on receipt of
	// a snap build labelled with a revision. The snap-declaration
	// type defines various snap properties, such as snap-id, its name,
	// and the publisher, plus policy related to accessing privileged
	// interfaces. The account assertion links an account name to
	// its identifier, and the account-key assertion holds the public
	// part of a key belonging to the account.
	var reason string
	if _, err := newSnapRevisionAssertionURLInfo(u); err == nil {
		reason = "valid URL for snap-revision assertion download"
	} else if _, err := newSnapDeclarationAssertionURLInfo(u); err == nil {
		reason = "valid URL for snap-declaration assertion download"
	} else if _, err := newAccountAssertionURLInfo(u); err == nil {
		reason = "valid URL for account-key assertion download"
	} else if _, err := newAccountKeyAssertionURLInfo(u); err == nil {
		reason = "valid URL for account-key assertion download"
	} else if _, err := newSerialAssertionURLInfo(u); err == nil {
		reason = "valid URL for serial assertion download"
	} else {
		return nil // we don't recognize this request
	}

	a.SetRequestPending(ins, reason)
	return nil
}

// InspectArtifact extracts metadata from a known artifact file format.
func (ins *SnapAssertionInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if a.ContentType() != "application/x.ubuntu.assertion" {
		return nil
	}

	if !a.MimetypeIs(mimetypes.Assertion) {
		return nil
	}

	sl := a.Logger()

	buf, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	md := ArtifactMetadata{
		Type: mimetypes.Assertion,
	}

	assert, err := newAssertion(buf)
	if err != nil {
		a.SetResponseRejected(ins, "error parsing assertion", md).Annotate(
			Annotation{"error-msg": err.Error()},
		)
		return nil
	}

	switch assert.Type() {
	case "snap-revision":
		md.Type = mimetypes.SnapRevisionAssertion
	case "snap-declaration":
		md.Type = mimetypes.SnapDeclarationAssertion
	case "account":
		md.Type = mimetypes.AccountAssertion
	case "account-key":
		md.Type = mimetypes.AccountKeyAssertion
	case "serial":
		md.Type = mimetypes.SerialAssertion
	}

	md.Name = "assertion"
	md.Description = fmt.Sprintf("%s assertion file", assert.Header["type"])
	md.Version = assert.Header["revision"]
	md.Vendor = assert.Header["authority-id"]
	md.Author = assert.Header["authority-id"]

	if err := assert.VerifySignature(sl); err != nil {
		a.SetResponseRejected(ins, "assertion signature verification failed", md).Annotate(
			Annotation{
				"assertion-type": assert.Type(),
				"error-msg":      err.Error(),
			},
		)
		return nil
	}

	notes := Annotation{}
	for k, v := range assert.Header {
		notes.Add(k, v)
	}

	a.SetResponseApproved(ins, "valid snap assertion", md).Annotate(notes)

	return nil
}
