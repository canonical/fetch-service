.. _ref_wheels:

.. meta::
    :description: Reference for the Python wheel inspector which verifies Python .whl archives.

The wheel inspector
===================

A Python wheel is a pre-built binary distribution format for Python
packages. Wheels are distributed as zip archives containing compiled code
and metadata files, and are published to PyPI-compatible repositories.

The wheel inspector examines and extracts metadata from Python wheels, as
published in the Python Package Index (PyPI).

Inspector ID
------------

``pip.wheel``

Internal state
--------------

None.

Request verification
--------------------

The wheel inspector recognizes requests to
``https://files.pythonhosted.org:443/packages/<hash-prefix>/<name>-<version>-*.whl``.

These requests are marked as unknown (unsupported origin) because
``files.pythonhosted.org`` is not a configured trusted origin.

File format
-----------

To be considered a wheel file, an artifact must meet the following criteria:

* It must be a zip archive
* It must contain a root directory ending in ``.dist-info``, containing
  at least the files called ``WHEEL``, ``METADATA``, and ``RECORD``.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the downloaded wheel file must also comply to
the following requirements:

* The ``METADATA`` file must contain at least the package name, package
  version, and metadata version. 
* The ``RECORD`` file must contain a comma-separated list of file name,
  file size, and SHA256 file digest encoded in base64 format.
* The wheel payload files must match the names, sizes, and SHA256 digests
  listed in the ``RECORD`` file.

Rejection reasons
-----------------

A wheel file can be rejected for the following reasons, in addition to
internal or environment errors:

* wheel file is corrupted or has missing elements
* wheel file parsed but failed integrity verification

Extracted metadata
------------------

The following pieces of metadata are extracted by the wheel inspector:

.. table:: Wheel inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.python.wheel``
   name          Yes   ``METADATA`` field ``name``
   version       Yes   ``METADATA`` field ``version``
   description   Yes   ``METADATA`` field ``summary``
   vendor        Yes   ``METADATA`` field ``author`` or ``maintainer``
   author        Yes   ``METADATA`` field ``author``
   author-email  Yes   ``METADATA`` field ``author-email``
   architecture
   license       Yes   ``METADATA`` using licensecheck
   ============  ====  ============================================

