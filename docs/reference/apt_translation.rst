The APT translation file inspector
==================================

The APT translation inspector verifies the APT repository's translation
files.

This inspector currently only examines the XZ-compressed version of
the Translation-<lang> files (they are the files download when running
``apt update`` on Ubuntu).

Inspector ID
------------

``apt.translations``

Internal state
--------------

None.

Request verification
--------------------

The APT packages inspector accepts HTTP requests to the Ubuntu archive,
including official ``*.archive.ubuntu.com`` mirrors, ``security.ubuntu.com``,
and HTTPS requests to ``esm.ubuntu.com``.

The translation file path is expected to match the regular expression
``^/ubuntu/dists/[\w-]+/[\w-]+/i18n/by-hash/SHA256/[0-9a-f]{64}$``.

File format
-----------

The APT release inspector expects the Translation-<lang> file to:

* Be a XZ-compressed text file.
* Contain blocks separated by a blank line, each block containing the
  fields ``Package``, ``Description-md5``, and ``Description-<lang>``.


Acceptance criteria
-------------------

To be approved, the artifacts examined by this inspector must comply
to the following rules:

* The ``Translation-<lang>.xz`` file must match the format described in the
  previous section.
* The deb file must match an entry in a previously downloaded Packages
  file.


Rejection reasons
-----------------

The ``Translation-<lang>.xz`` file is rejected if:

* Expected fields are missing.


Extracted metadata
------------------

The following pieces of metadata are extracted by the APT packages file
 inspector:

.. table:: APT release inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.apt.translation``
   name          Yes   ``Translation-<lang>``
   version       Yes   ``InRelease`` field ``Codename``
   description
   vendor        Yes   ``InRelease`` field ``Origin``
   author        Yes   ``InRelease`` field ``Origin``
   author-email
   architecture                   
   license
   copyright                   
   ============  ====  ============================================

