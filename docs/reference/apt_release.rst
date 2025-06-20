The APT release file inspector
==============================

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

The APT release inspector accepts HTTP requests to the Ubuntu archive,
including official ``*.archive.ubuntu.com`` mirrors, ``security.ubuntu.com``,
and HTTPS requests to ``esm.ubuntu.com``.

The InRelease file path is expected to match the regular expression
``^/ubuntu/dists/[\w-]+/InRelease$``.

File format
-----------

The APT release inspector ensures the InRelease file:

* Is a GPG-clearsigned text file, signed by the Ubuntu archive ftpmaster.
* Contains the fields ``Origin``, ``Label``, ``Suite``, ``Version``,
  ``Codename``, ``Date``, ``Architectures``, ``Components``, and
  ``Description``.
* It contains a ``SHA256`` section containing lines in the format
  <digest> <size> <filename> with whitespace separators.


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

* They're download before the repository's ``InRelease`` file.
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
   license
   copyright                   
   ============  ====  ============================================

