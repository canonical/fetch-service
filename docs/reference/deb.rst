.. _ref_deb:

.. meta::
    :description: Reference for the Debian package inspector which verifies .deb binary packages from APT repositories.

The Debian package inspector
============================

A Debian binary package (``.deb``) is an AR archive containing a package's
compiled programs, libraries, and configuration files together with control
metadata. The ``.deb`` format is the standard binary distribution format for
Debian-based Linux distributions such as Ubuntu.

The Debian package inspector examines ``.deb`` binary packages downloaded
from APT repositories.

Inspector ID
------------

``deb``

Internal state
--------------

None.

Request verification
--------------------

The Debian package inspector accepts HTTP requests to the repositories
configured in the ``apt.repositories.<name>`` entry in ``inspectors.yaml``.

The Debian package URL is expected to match the path pattern for ``.deb``
files in an APT pool directory:
``/<repo>/pool/<component>/<prefix>/<source>/<name>_<version>_<arch>.deb``.

File format
-----------

The Debian package inspector expects the artifact to be an AR archive
(``debian-binary`` version 2.0) containing:

* A ``control.tar.*`` member (compressed with gz, xz, zst, or uncompressed)
  containing a ``./control`` file with the fields ``Package``, ``Version``,
  ``Architecture``, ``Description``, ``Maintainer``, and optionally ``Source``.
* A ``data.tar.*`` member (compressed with gz, xz, zst, or uncompressed)
  containing the package payload including a ``copyright`` file at
  ``./usr/share/doc/<name>/copyright``.

Configuration options
---------------------

This inspector is configured under the ``apt`` key in ``inspectors.yaml``,
sharing the same repository configuration as the
:doc:`apt.release <apt_release>` inspector. Only the ``urls`` option from
each ``apt.repositories.<name>`` entry is used to match request URLs to
known APT repositories.

Acceptance criteria
-------------------

To be approved, the artifact must:

* Parse as a valid Debian binary package.
* Be listed in a previously downloaded and approved Packages file
  (verified by the :doc:`apt.packages <apt_packages>` inspector).

Rejection reasons
-----------------

The artifact is rejected if:

* The ``debian-binary`` version is not ``2.0``.
* The control metadata cannot be parsed.
* The ``Package`` name and ``Version`` fields are missing.
* No previously approved Packages file covers this package.
* The Packages file covering this package was itself rejected.

Extracted metadata
------------------

The following pieces of metadata are extracted by the Debian package
inspector:

.. table:: Debian package inspector metadata
   :widths: auto

   ==============  ====  ============================================
   Field           Used  Data source
   ==============  ====  ============================================
   type            Yes   ``application/vnd.debian.binary-package``
   name            Yes   ``control`` field ``Package``
   version         Yes   ``control`` field ``Version``
   description     Yes   ``control`` field ``Description``
   architecture    Yes   ``control`` field ``Architecture``
   vendor          Yes   ``control`` field ``Maintainer``
   author          Yes   ``copyright`` field ``Upstream author``
   license         Yes   ``copyright`` file using licensecheck
   source-package  Yes   ``control`` field ``Source``
   ==============  ====  ============================================
