The APT commands file inspector
===============================

The APT commands inspector verifies the APT repository's Commands
files.

This inspector currently only examines the XZ-compressed version of
the commands-<arch>.xz files.

Inspector ID
------------

``apt.commands``

Internal state
--------------

None.

Request verification
--------------------

The APT commands inspector accepts HTTP requests to the Ubuntu archive,
including official ``*.archive.ubuntu.com`` mirrors, ``security.ubuntu.com``,
and HTTPS requests to ``esm.ubuntu.com``.

The commands file path is expected to match the regular expression
``^/ubuntu/dists/[\w-]+/[\w-]+/cnf/by-hash/SHA256/[0-9a-f]{64}$``.

File format
-----------

The APT commands inspector expects the Commands-<arch>.xz file to:

* Be a XZ-compressed text file.
* Contain a header block with `suite`, `component`, and `arch` entries.
* Contain blocks separated by a blank line, each block containing the
  fields ``name``, ``version``, and ``commands``.


Acceptance criteria
-------------------

To be approved, the artifacts examined by this inspector must comply
to the following rules:

* The ``Commands-<lang>.xz`` file must match the format described in the
  previous section.
* The deb file must match an entry in a previously downloaded Packages
  file.


Rejection reasons
-----------------

The ``Commands-<lang>.xz`` file is rejected if:

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
   type          Yes   ``application/x.apt.commands``
   name          Yes   ``Commands-<lang>``
   version       Yes   ``InRelease`` field ``Codename``
   description
   vendor        Yes   ``InRelease`` field ``Origin``
   author        Yes   ``InRelease`` field ``Origin``
   author-email
   architecture
   license
   copyright
   ============  ====  ============================================

