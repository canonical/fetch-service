.. _ref_sdist:

.. meta::
    :description: Reference for the Python source distribution inspector which verifies Python sdist archives.

The sdist inspector
===================

A Python source distribution (sdist) is a compressed archive of a Python
package's source code together with a ``PKG-INFO`` metadata file. Sdists
are published to PyPI-compatible repositories as a source-based alternative
to pre-built wheel packages.

The sdist inspector examines and extracts metadata from Python sdists, as
published in the Python Package Index (PyPI).

Inspector ID
------------

``pip.sdist``

Internal state
--------------

None.

Request verification
--------------------

The sdist inspector recognizes requests to
``https://files.pythonhosted.org:443/packages/<hash-prefix>/<name>-<version>-*.tar.gz``.

These requests are marked as unknown (unsupported origin) because
``files.pythonhosted.org`` is not a configured trusted origin.

File format
-----------

To be considered an sdist file, an artifact must meet the following criteria:

* It must be a gzipped tar archive.
* It must contain a ``PKG-INFO`` file in the root directory.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the downloaded sdist file must also meet
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
   type          Yes   ``application/x.python.sdist``
   name          Yes   ``PKG-INFO`` field ``name``
   version       Yes   ``PKG-INFO`` field ``version``
   description   Yes   ``PKG-INFO`` field ``summary``
   vendor        Yes   ``PKG-INFO`` field ``author`` or ``maintainer``
   author        Yes   ``PKG-INFO`` field ``author``
   author-email  Yes   ``PKG-INFO`` field ``author-email``
   architecture
   license       Yes   ``PKG-INFO`` using licensecheck
   ============  ====  ============================================
