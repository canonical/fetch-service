The Python metadata inspector
=============================

The Python metadata inspector examines the metadata file as published in
the Python Package Index (PyPI).

Inspector ID
------------

``pip.metadata``

Internal state
--------------

None

Request verification
--------------------

The current implementation allows downloads from the PyPI archive. This
will be changed to match an internal repository of binary artefacts. The
downloaded file must have the ``.metadata`` suffix.

File format
-----------

To be considered a Python metadata file, an artefact must meet the
following criteria:

* It must be a plain text file.
* It must contain key: value lines.

Acceptance criteria
-------------------

In order be approved, the downloaded metadata file must also meet the
following requirements:

* The file must contain at least the package name, package version, and
  metadata version. 

Rejection reasons
-----------------

A metadata file can be rejected for the following reasons, in addition to
internal or environment errors:

* Python metadata file is corrupted or has missing elements.

Extracted metadata
------------------

The following pieces of metadata are extracted by the Python metadata inspector:

.. table:: Python metadata file inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   name          Yes   metadata field ``name``
   version       Yes   metadata field ``version``
   description   Yes   "metadata file for package <name>"
   vendor        Yes   metadata field ``maintainer`` or ``author``
   author        Yes   metadata field ``author``
   author-email  Yes   metadata field ``author-email``
   architecture
   license
   copyright
   ============  ====  ============================================
