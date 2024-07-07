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

package git

import (
	"fmt"
	"net/url"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

type SmartQueryInspector struct {
}

func NewSmartQueryInspector() *SmartQueryInspector {
	return &SmartQueryInspector{}
}

func (SmartQueryInspector) ID() string {
	return "git.smart-query"
}

func (ins *SmartQueryInspector) InspectRequest(a RequestArtefact) error {
	proto := getGitProtocol(a)
	if proto != "version=2" {
		return nil
	}

	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if info, err := newSmartQueryUrlInfo(u); err == nil {
		a.SetRequestPending(ins, "valid URL for git smart request").Annotate(
			Annotation{
				"protocol": proto,
				"service":  info.service,
			},
		)
	}

	return nil // we don't recognize this request
}

func (ins *SmartQueryInspector) InspectArtefact(f ArtefactFile, a ResponseArtefact) error {
	if !a.MimetypeIs("text/plain") {
		return nil
	}

	if a.ContentType() != "application/x-git-upload-pack-advertisement" {
		return nil
	}

	// Content type says it's an upload pack advertisement
	a.SetArtefactMetadata(ArtefactMetadata{
		Type: mimetypes.GitUploadPackAdvertisement,
		Name: "git upload-pack advertisement",
	})

	msgs, err := decodeGitProtocol(f)
	if err != nil {
		a.SetResponseRejected(ins, "cannot decode git V2 protocol").Annotate(
			Annotation{
				"error-msg": err.Error(),
			})
		return nil
	}

	if len(msgs) == 1 && msgs[0] == "# service=git-upload-pack\n" {
		var err error
		msgs, err = decodeGitProtocol(f) // skip previous size+content
		if err != nil {
			a.SetResponseRejected(ins, "cannot decode pack advertisement").Annotate(
				Annotation{
					"error-msg": err.Error(),
				})
			return nil
		}
	}

	if len(msgs) < 1 || msgs[0] != "version 2\n" {
		a.SetResponseRejected(ins, "git protocol is not version 2")
		return nil
	}

	var server_msgs []string
	for _, msg := range msgs {
		server_msgs = append(server_msgs, strings.TrimSpace(msg))
	}

	// A server which decides to communicate (based on a request from a client)
	// using protocol version 2 notifies the client by sending a version string
	// in its initial response followed by an advertisement of its capabilities.
	// Each capability is a key with an optional value.
	a.SetResponseApproved(ins, "upload pack advertisement received").Annotate(
		Annotation{"server-response": server_msgs},
	)
	return nil
}
