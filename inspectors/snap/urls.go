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
	"net/url"
	"regexp"
)

// Recognized URL formats:
// -----------------------
// https://canonical-bos01.cdn.snapcraftcontent.com:443/download-origin/canonical-lgw01/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7_7.snap?...
// https://api.snapcraft.io:443/v2/snaps/info/word-salad?...
// https://api.snapcraft.io:443/v2/snaps/refresh
// https://api.snapcraft.io:443/v2/assertions/snap-revision/Gsqj3QgpWFq2p0517nHNNMZWgX5rG6_vaeOjT9Nyua_l36qkC_HdiDw2iEd4t1J-?...
// https://api.snapcraft.io:443/v2/assertions/snap-declaration/16/CSO04Jhav2yK0uz97cr0ipQRyqg0qQL6?...
// https://api.snapcraft.io:443/v2/assertions/account/ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG?..."
// https://api.snapcraft.io:443/v2/assertions/account-key/BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul?...
// https://api.snapcraft.io:443/api/v1/snaps/sections
// https://api.snapcraft.io:443/api/v1/snaps/auth/sessions
// https://api.snapcraft.io:443/api/v1/snaps/auth/nonces
// https://api.snapcraft.io:443/api/v1/snaps/names
// https://api.snapcraft.io:443/api/v1/snaps/auth/request-id
// https://api.snapcraft.io:443/api/v1/snaps/auth/devices/

var (
	reSnapPackage    = regexp.MustCompile(`^https://api.snapcraft.io:443/api/v1/snaps/download/([A-Za-z0-9]+)_([0-9]+)\.snap`)
	reSnapPackageAlt = regexp.MustCompile(`^https://[^/]+\.snapcraftcontent\.com:443/[^?]+/([A-Za-z0-9]+)_([0-9]+)\.snap\?`)
	reSnapInfo       = regexp.MustCompile(`^https://api.snapcraft.io:443/v2/snaps/info/([^?/]+)`)
	reSnapRefresh    = regexp.MustCompile(`^https://api.snapcraft.io:443/v2/snaps/refresh$`)

	reSnapRevisionAssertion    = regexp.MustCompile(`^https://api.snapcraft.io:443/v2/assertions/snap-revision/`)
	reSnapDeclarationAssertion = regexp.MustCompile(`^https://api.snapcraft.io:443/v2/assertions/snap-declaration/`)
	reAccountAssertion         = regexp.MustCompile(`^https://api.snapcraft.io:443/v2/assertions/account/`)
	reAccountKeyAssertion      = regexp.MustCompile(`^https://api.snapcraft.io:443/v2/assertions/account-key/`)
	reSerialAssertion          = regexp.MustCompile(`^https://api.snapcraft.io:443/api/v1/snaps/auth/devices/?$`)

	reSnapSections      = regexp.MustCompile(`^https://api.snapcraft.io:443/api/v1/snaps/sections$`)
	reSnapNames         = regexp.MustCompile(`^https://api.snapcraft.io:443/api/v1/snaps/names(?:\?.*)?$`)
	reSnapAuthSessions  = regexp.MustCompile(`^https://api.snapcraft.io:443/api/v1/snaps/auth/sessions$`)
	reSnapAuthNonce     = regexp.MustCompile(`^https://api.snapcraft.io:443/api/v1/snaps/auth/nonces$`)
	reSnapAuthRequestID = regexp.MustCompile(`^https://api.snapcraft.io:443/api/v1/snaps/auth/request-id$`)
)

type snapPackageURLInfo struct {
	snapID  string
	release string
}

func newSnapPackageURLInfo(u *url.URL) (*snapPackageURLInfo, error) {
	m := reSnapPackage.FindStringSubmatch(u.String())
	if len(m) != 3 {
		m = reSnapPackageAlt.FindStringSubmatch(u.String())
		if len(m) != 3 {
			return nil, fmt.Errorf("%s: not a valid snap package URL", u.Path)
		}
	}
	info := &snapPackageURLInfo{
		snapID:  m[1],
		release: m[2],
	}
	return info, nil
}

type snapInfoURLInfo struct {
	name string
}

func newSnapInfoURLInfo(u *url.URL) (*snapInfoURLInfo, error) {
	m := reSnapInfo.FindStringSubmatch(u.String())
	if len(m) != 2 {
		return nil, fmt.Errorf("%s: not a valid snap info URL", u.Path)
	}
	info := &snapInfoURLInfo{
		name: m[1],
	}
	return info, nil
}

type snapRefreshURLInfo struct {
}

func newSnapRefreshURLInfo(u *url.URL) (*snapRefreshURLInfo, error) {
	if !reSnapRefresh.MatchString(u.String()) {
		return nil, fmt.Errorf("%s: not a valid snap refresh URL", u.String())
	}
	info := &snapRefreshURLInfo{}
	return info, nil
}

type snapRevisionAssertionURLInfo struct {
}

func newSnapRevisionAssertionURLInfo(u *url.URL) (*snapRevisionAssertionURLInfo, error) {
	if !reSnapRevisionAssertion.MatchString(u.String()) {
		return nil, fmt.Errorf("%s: not a valid snap-revision assertion URL", u.Path)
	}
	info := &snapRevisionAssertionURLInfo{}
	return info, nil
}

type snapDeclarationAssertionURLInfo struct {
}

func newSnapDeclarationAssertionURLInfo(u *url.URL) (*snapDeclarationAssertionURLInfo, error) {
	if !reSnapDeclarationAssertion.MatchString(u.String()) {
		return nil, fmt.Errorf("%s: not a valid snap-declaration assertion URL", u.Path)
	}
	info := &snapDeclarationAssertionURLInfo{}
	return info, nil
}

type accountAssertionURLInfo struct {
}

func newAccountAssertionURLInfo(u *url.URL) (*accountAssertionURLInfo, error) {
	if !reAccountAssertion.MatchString(u.String()) {
		return nil, fmt.Errorf("%s: not a valid account assertion URL", u.Path)
	}
	info := &accountAssertionURLInfo{}
	return info, nil
}

type accountKeyAssertionURLInfo struct {
}

func newAccountKeyAssertionURLInfo(u *url.URL) (*accountKeyAssertionURLInfo, error) {
	if !reAccountKeyAssertion.MatchString(u.String()) {
		return nil, fmt.Errorf("%s: not a valid account-key assertion URL", u.Path)
	}
	info := &accountKeyAssertionURLInfo{}
	return info, nil
}

type snapSectionsURLInfo struct {
}

func newSnapSectionsURLInfo(u *url.URL) (*snapSectionsURLInfo, error) {
	if !reSnapSections.MatchString(u.String()) {
		return nil, fmt.Errorf("%s: not a valid snap sections URL", u.Path)
	}
	info := &snapSectionsURLInfo{}
	return info, nil
}

type snapAuthSessionsURLInfo struct {
}

func newSnapAuthSessionsURLInfo(u *url.URL) (*snapAuthSessionsURLInfo, error) {
	if !reSnapAuthSessions.MatchString(u.String()) {
		return nil, fmt.Errorf("%s: not a valid auth sessions URL", u.Path)
	}
	info := &snapAuthSessionsURLInfo{}
	return info, nil
}

type snapAuthNonceURLInfo struct {
}

func newSnapAuthNonceURLInfo(u *url.URL) (*snapAuthNonceURLInfo, error) {
	if !reSnapAuthNonce.MatchString(u.String()) {
		return nil, fmt.Errorf("%s: not a valid auth nonce URL", u.Path)
	}
	info := &snapAuthNonceURLInfo{}
	return info, nil
}

type snapAuthRequestIDURLInfo struct {
}

func newSnapAuthRequestIDURLInfo(u *url.URL) (*snapAuthRequestIDURLInfo, error) {
	if !reSnapAuthRequestID.MatchString(u.String()) {
		return nil, fmt.Errorf("%s: not a valid device authentication request-id URL", u.Path)
	}
	info := &snapAuthRequestIDURLInfo{}
	return info, nil
}

type snapNamesURLInfo struct {
}

func newSnapNamesURLInfo(u *url.URL) (*snapNamesURLInfo, error) {
	if !reSnapNames.MatchString(u.String()) {
		return nil, fmt.Errorf("%s: not a valid snap names URL", u.Path)
	}
	info := &snapNamesURLInfo{}
	return info, nil
}

type serialAssertionURLInfo struct {
}

func newSerialAssertionURLInfo(u *url.URL) (*serialAssertionURLInfo, error) {
	if !reSerialAssertion.MatchString(u.String()) {
		return nil, fmt.Errorf("%s: not a valid serial assertion URL", u.Path)
	}
	info := &serialAssertionURLInfo{}
	return info, nil
}
