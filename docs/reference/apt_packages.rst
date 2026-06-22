.. _ref_apt_packages:

.. meta::
    :description: Reference for the APT packages file inspector which verifies Packages files listing available packages in an APT repository.

The APT packages file inspector
===============================

APT Packages files list the packages available in an APT repository along
with their metadata and SHA256 checksums. They are downloaded by package
managers such as ``apt`` during a repository refresh.

The APT packages inspector verifies the APT repository's Packages
file and whether it has a matching entry for downloaded deb files.

This inspector examines the gzip-compressed version of the Packages file
as well as files fetched by hash (which may be XZ or gzip compressed).

Inspector ID
------------

``apt.packages``

Internal state
--------------

* Essential information about each package listed in the Packages
  file, namely the name, version, size, architecture, and corresponding
  SHA256 digest of the Packages.xz file for each repository.

Request verification
--------------------

The APT packages inspector accepts HTTP requests to the repositories
configured in the ``apt.repositories.<name>`` entry in ``inspectors.yaml``.

The Packages file path is expected to match one of these patterns:

* Direct: ``/<repo>/dists/[\w-]+/[\w-]+/binary-\w+/Packages.gz``
* By hash: ``/<repo>/dists/[\w-]+/[\w-]+/binary-\w+/by-hash/SHA256/[0-9a-f]{64}``

File format
-----------

The APT packages inspector expects the Packages file to:

* Be a compressed (gzip or XZ) text file.
* Contain blocks separated by a blank line, each block containing the
  fields ``Package``, ``Architecture``, ``Version``, ``Priority``,
  ``Section``, and ``Maintainer``.


Configuration options
---------------------

This inspector is configured under the ``apt`` key in ``inspectors.yaml``.
The ``repositories`` map defines the allowed APT repositories.

.. list-table:: Per-repository options (``apt.repositories.<name>``)
   :widths: auto
   :header-rows: 1

   * - Option
     - Description
   * - ``urls``
     - List of URL glob patterns for the repository base URL.
   * - ``suites``
     - List of allowed suite name glob patterns (for example, ``focal``
       or ``noble-updates``).
   * - ``components``
     - List of allowed component glob patterns (for example, ``main``
       or ``universe``).

Acceptance criteria
-------------------

To be approved, the artifacts examined by this inspector must comply
to the following rules:

* The ``Packages`` file must follow the format described in the
  previous section.

When a deb file is downloaded and matches an entry in a previously downloaded
Packages file, this inspector marks it as unknown (pending further
validation by the :doc:`deb <deb>` inspector).


Rejection reasons
-----------------

The ``Packages`` file is rejected if:

* The package entries cannot be parsed.

The deb file is rejected if:

* It doesn't correspond to an entry in a previously downloaded Packages file.
* The file size does not match the Packages entry.
* The URL architecture does not match the Packages entry.


Extracted metadata
------------------

The following pieces of metadata are extracted by the APT packages file
inspector:

.. table:: APT packages file inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.apt.packages``
   name          Yes   ``Packages``
   version
   description   Yes   ``<Suite> <Component> Packages file``
   vendor        Yes   ``InRelease`` field ``Origin``
   author        Yes   ``InRelease`` field ``Origin``
   author-email
   architecture  Yes   Binary architecture from request URL
   ============  ====  ============================================

