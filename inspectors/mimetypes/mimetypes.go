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

package mimetypes

const (
	DebianBinaryPackage        = "application/vnd.debian.binary-package"
	PythonWheel                = "application/x.python.wheel"
	PythonSdist                = "application/x.python.sdist"
	PythonMetadata             = "application/x.python.metadata"
	AptRelease                 = "application/x.apt.release"
	AptPackages                = "application/x.apt.packages"
	AptTranslation             = "application/x.apt.translation"
	AptCommands                = "application/x.apt.commands"
	GitUploadPackAdvertisement = "application/x.git.upload-pack-advertisement"
	GitUploadPackLsRef         = "application/x.git.upload-pack-result.ls-ref"
	GitUploadPackFetch         = "application/x.git.upload-pack-result.fetch"
	Charmcraft                 = "application/x.canonical.charmcraft"
	Rockcraft                  = "application/x.canonical.rockcraft"
	Snapcraft                  = "application/x.canonical.snapcraft"
	Sourcecraft                = "application/x.canonical.sourcecraft"
	SimpleStreams              = "application/x.canonical.simplestreams"
	SimpleStreamsProduct       = "application/x.canonical.simplestreams-product"
	GoModuleGit                = "application/x.go.module.git-repo"
	SquashFs                   = "application/x.squashfs"
	SnapPackage                = "application/x.canonical.snap-package"
	SnapRefresh                = "application/x.canonical.snap-refresh"
	SnapInfo                   = "application/x.canonical.snap-info"
	Assertion                  = "application/x.ubuntu.assertion"
	SnapRevisionAssertion      = "application/x.ubuntu.assertion.snap-revision"
	SnapDeclarationAssertion   = "application/x.ubuntu.assertion.snap-declaration"
	AccountAssertion           = "application/x.ubuntu.assertion.account"
	AccountKeyAssertion        = "application/x.ubuntu.assertion.account-key"
	RustCrate                  = "application/x.rust.crate"
	StoreAPI                   = "application/x.canonical.store-api"
	BldBinPackage              = "application/x.canonical.bld-bin-package"
	LxdRootfs                  = "application/x.canonical.lxd-rootfs"
)
