The wheel inspector
===================

The wheel inspector examines and extracts metadata from Python wheels, as
published in the Python Package Index (PyPI).

Inspector ID
------------

``pip.wheel``

Internal state
--------------

None

Request verification
--------------------

The current implementation allows downloads from the PyPI archive. This
will be changed to match an internal repository of binary artefacts.

File format
-----------

To be considered a wheel file, an artefact must meet the following criteria:

* It must be a zip archive
* It must contain a root directory ending in ``dist-info``, containing
  at least the files called ``WHEEL``, ``METADATA``, and ``RECORD``.

Acceptance criteria
-------------------

In order be approved, the the downloaded wheel file must also comply to
the following requirements:

* The ``METADATA`` file must contain at least the package name, package
  version, and metadata version. 
* The ``RECORD```file must contain a comma-separated list of file name,
  file size, and SHA256 file digest encoded in base64 format.
* The wheel payload files must match the names, sizes, and SHA256 digests
  listed in the ``RECORD`` file.

Rejection reasons
-----------------

A wheel file can be rejected reasons excluding internal or environmental
errors:

* wheel file requirements not met
* wheel file parsed but failed integrity verification

Extracted metadata
------------------

The following pieces of metadata are extracted by the wheel inspector:

.. table:: Wheel inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   name          Yes   ``METADATA`` field ``name``
   version       Yes   ``METADATA`` field ``version``
   description   Yes   ``METADATA`` field ``summary``
   vendor        Yes   ``METADATA`` field ``maintainer`` or ``author``
   author        Yes   ``METADATA`` field ``author``
   author-email  Yes   ``METADATA`` field ``author-email``
   architecture                   
   license       Yes   ``METADATA`` via licensecheck
   copyright                   
   ============  ====  ============================================

