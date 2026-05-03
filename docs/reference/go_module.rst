.. _ref_go_module:

.. meta::
    :description: Reference for the Go module Git inspector which verifies that a cloned Git repository contains a valid Go module.

The Go module Git inspector
===========================

A Go module is a collection of related Go packages with a defined module
path and version. Module source code is hosted in Git repositories and
described by a ``go.mod`` file in the repository root.

The Go module Git inspector checks whether the Git repository fetched through
the git upload-pack protocol contains a valid Go module.

This inspector must be called after the :doc:`git.upload-pack
<git_upload_pack>` inspector, and only supports v2 of the `smart transfer
protocol
<https://git-scm.com/book/en/v2/Git-Internals-Transfer-Protocols#_the_smart_protocol>`_
in Git.

Inspector ID
------------

``go.module.git``

Internal state
--------------

None.

Request verification
--------------------

The Go module Git inspector currently has an empty list of valid origins,
so ``InspectRequest`` never recognizes any request and sets no opinion.

When valid origins are configured, a request will be marked as unknown
(unsupported origin) when all the following conditions are met:

* The ``Content-Type`` header is ``application/x-git-upload-pack-request``.
* The ``Accept`` header is ``application/x-git-upload-pack-result``.
* The URL path matches a Go module upload-pack pattern (ending in
  ``/git-upload-pack``).
* The upload-pack command (set by the :doc:`git.upload-pack <git_upload_pack>` inspector) is
  ``fetch``.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

The repository is approved if:

* The git checkout (performed by the :doc:`git.upload-pack <git_upload_pack>` inspector) contains
  a ``go.mod`` file in the repository root.
* The ``go.mod`` file can be parsed and the module name is non-empty.
* At least one of the fetched Git tags matches the requested revision.
* A license file (``LICENSE`` or ``COPYING``) is optionally found.

Rejection reasons
-----------------

The artifact is deferred (unknown) if:

* No git checkout path was set by the :doc:`git.upload-pack <git_upload_pack>` inspector.
* The ``go.mod`` file cannot be parsed.
* No version tag matching the fetched revision can be found.

If the repository does not contain a ``go.mod`` file,
the inspector returns no verdict and defers to subsequent inspectors.

Extracted metadata
------------------

The following pieces of metadata are extracted by the Go module Git inspector:

.. table:: Go module Git inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.go.module.git-repo``
   name          Yes   Last path segment of ``go.mod`` module name
   version       Yes   Git tag matching the fetched revision
   vendor        Yes   Second-to-last path segment of module name
   description
   author
   license       Yes   ``LICENSE`` or ``COPYING`` file using licensecheck
   ============  ====  ============================================
