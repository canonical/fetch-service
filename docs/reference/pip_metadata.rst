.. _ref_pip_metadata:

.. meta::
    :description: Reference for the Python metadata inspector which verifies Python package metadata files from a package repository.

The Python metadata inspector
=============================

A Python metadata file (with the ``.metadata`` extension) contains
structured package metadata for a Python distribution, following the Python
packaging metadata specification. Metadata files are published alongside
wheel files in PyPI-compatible repositories.

The Python metadata inspector examines the metadata file as published in
the Python Package Index (PyPI).

Inspector ID
------------

``pip.metadata``

Internal state
--------------

None.

Request verification
--------------------

The Python metadata inspector recognizes requests to
``https://files.pythonhosted.org:443/packages/<hash-prefix>/<name>-<version>-*.metadata``.

These requests are marked as unknown (unsupported origin) because
``files.pythonhosted.org`` is not a configured trusted origin.

File format
-----------

To be considered a Python metadata file, an artifact must meet the
following criteria:

* It must be a plain text file.
* It must contain key: value lines.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the downloaded metadata file must also meet the
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
   type          Yes   ``application/x.python.metadata``
   name          Yes   ``metadata file for package '<name>'``
   version       Yes   metadata field ``version``
   description   Yes   ``Python metadata file``
   vendor        Yes   metadata field ``author`` or ``maintainer``
   author        Yes   metadata field ``author``
   author-email  Yes   metadata field ``author-email``
   architecture
   ============  ====  ============================================
