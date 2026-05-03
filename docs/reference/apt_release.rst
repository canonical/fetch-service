.. _ref_apt_release:

.. meta::
    :description: Reference for the APT release file inspector which verifies InRelease files and authenticates the index files of an APT repository.

The APT release file inspector
==============================

APT InRelease files are GPG-clearsigned index files that serve as the trust
anchor for an APT repository. They contain checksums for all other
repository metadata files, allowing clients to verify the integrity of
downloaded packages.

The APT release inspector examines different types of artifacts. Besides
the Ubuntu archive InRelease file, it verifies whether Packages.xz, Translation,
and Commands files are correctly listed in the InRelease file with a matching
file digest.

Inspector ID
------------

``apt.release``

Internal state
--------------

* Release entries per repository containing SHA256 digest, size, and
  name of APT metadata files.

Request verification
--------------------

The APT release inspector accepts HTTP requests to the repositories
configured in the ``apt.repositories.<name>`` entry in ``inspectors.yaml``.

The InRelease file path is expected to match the regular expression
``/<repo>/dists/[\w-]+/InRelease``.

File format
-----------

The APT release inspector ensures the InRelease file:

* Is a GPG-clearsigned text file, signed by the Ubuntu archive ftpmaster.
* Contains the fields ``Origin``, ``Label``, ``Suite``, ``Version``,
  ``Codename``, ``Date``, ``Architectures``, and ``Components``.
* It contains a ``SHA256`` section containing lines in the format
  <digest> <size> <filename> with whitespace separators.


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
   * - ``public-key``
     - PGP public key block used to verify the ``InRelease`` file signature.
   * - ``base-url-alias``
     - Optional. Maps the repository origin to a different base URL in
       internal state records.

Acceptance criteria
-------------------

To be approved, the artifacts examined by this inspector must comply
to the following rules:

* The ``InRelease`` file must have the previously described file format.
* The ``Packages.xz`` file must match an entry in the ``InRelease`` file
  of the same repository.
* The ``Translation-<lang>.xz`` file must match an entry in the
  ``InRelease`` file of the same repository.
* The ``Commands-<arch>.xz`` file must match an entry in the ``InRelease``
  file of the same repository.


Rejection reasons
-----------------

The ``InRelease`` file is rejected if:

* The URL doesn't match the Ubuntu archive hosts and file paths.
* The expected fields are not found.
* The file entries cannot be parsed.

The ``Packages.xz`, ``Translation-<lang>.xz``, or ``Commands-<arch>.xz``
files are rejected if:

* They're downloaded before the repository's ``InRelease`` file.
* The files don't match the listing in the repository's ``InRelease``
  file.


Extracted metadata
------------------

The following pieces of metadata are extracted by the APT release
inspector:

.. table:: APT release inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.apt.release``
   name          Yes   ``InRelease``
   version       Yes   ``InRelease`` field ``Codename``
   description   Yes   ``InRelease`` field ``Description``
   vendor        Yes   ``InRelease`` field ``Origin``
   author        Yes   ``InRelease`` field ``Origin``
   author-email
   architecture                   
   ============  ====  ============================================

