The sdist inspector
===================

The sdist inspector examines and extracts metadata from Python sdists, as
published in the Python Package Index (PyPI).

Inspector ID
------------

``pip.sdist``

Internal state
--------------

None

Request verification
--------------------

The current implementation allows downloads from the PyPI archive. This
will be changed to match an internal repository of binary artifacts.

File format
-----------

To be considered an sdist file, an artifact must meet the following criteria:

* It must be a gzipped tar archive.
* It must contain a ``PKG-INFO`` file in the root directory.

Acceptance criteria
-------------------

In order be approved, the downloaded sdist file must also meet
the following requirements:

* The ``PKG-INFO`` file must contain at least the package name, package
  version, and metadata version. 

Rejection reasons
-----------------

An sdist file can be rejected for the following reasons, in addition to
internal or environment errors:

* sdist file is corrupted or has missing elements.

Extracted metadata
------------------

The following pieces of metadata are extracted by the sdist inspector:

.. table:: sdist inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   name          Yes   ``PKG-INFO`` field ``name``
   version       Yes   ``PKG-INFO`` field ``version``
   description   Yes   ``PKG-INFO`` field ``summary``
   vendor        Yes   ``PKG-INFO`` field ``maintainer`` or ``author``
   author        Yes   ``PKG-INFO`` field ``author``
   author-email  Yes   ``PKG-INFO`` field ``author-email``
   architecture                   
   license       Yes   ``PKG-INFO`` via licensecheck
   copyright                   
   ============  ====  ============================================
