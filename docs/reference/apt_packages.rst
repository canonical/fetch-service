The APT packages file inspector
===============================

The APT packages inspector verifies the APT repository's Packages.xz
file and whether it has a matching entry for downloaded deb files.

This inspector currently only examines the XZ-compressed version of
the Packages file (it's the file downloaded when running ``apt update``
on Ubuntu).

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

The apt packages inspector accepts HTTP requests to the Ubuntu archive,
including official ``*.archive.ubuntu.com`` mirrors, ``security.ubuntu.com``,
and HTTPS requests to ``esm.ubuntu.com``.

The Packages.xz file path is expected to match the regular expression
``^/ubuntu/dists/[\w-]+/[\w-]+/binary-\w+/by-hash/SHA256/[0-9a-f]{64}$``

File format
-----------

The APT release inspector expects the Packages file to:

* Be a XZ-compressed text file.
* Contain blocks separated by a blank line, each block containing the
  fields ``Package``, ``Architecture``, ``Version", ``Priority``,
  ``Section``, ``Origin``, and ``Maintainer``.


Acceptance criteria
-------------------

To be approved, the artefacts examined by this inspector must comply
to the following rules:

* The ``Packages.xz`` file must match the format described in the
  previous section.
* The deb file must match an entry in a previously downloaded Packages
  file.


Rejection reasons
-----------------

The ``Packages.xz`` file is rejected if:

* The package entries cannot be parsed.

The deb file is rejected if:

* It doesn't correspond to an entry in a previously downloaded Packages
  file.


Extracted metadata
------------------

The following pieces of metadata are extracted by the APT packages file
 inspector:

.. table:: APT release inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.apt.packages``
   name          Yes   ``Packages``
   version       Yes   ``InRelease`` field ``Codename``
   description   Yes   ``<Codename> <Component> Packages file``
   vendor        Yes   ``InRelease`` field ``Origin``
   author        Yes   ``InRelease`` field ``Origin``
   author-email
   architecture                   
   license
   copyright                   
   ============  ====  ============================================

