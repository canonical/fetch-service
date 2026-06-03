.. _ref_pip_simple_index:

.. meta::
    :description: Reference for the PyPI simple index inspector which verifies Simple Index JSON responses from a Python package repository.

The PyPI simple index inspector
================================

The PyPI Simple Repository API is a protocol for discovering Python
packages. For each package, the simple index lists all available
distribution files together with their integrity hashes.

The PyPI simple index inspector examines the PyPI Simple Repository API
index responses for individual packages.

Inspector ID
------------

``pip.simple-index``

Internal state
--------------

None.

Request verification
--------------------

The PyPI simple index inspector recognizes requests to
``https://pypi.org:443/simple/<package-name>/``.

These requests are marked as unknown (unsupported origin)
because ``pypi.org`` is not a configured trusted origin.

File format
-----------

The inspector recognizes two response formats:

* **HTML** (``text/html``): contains a ``<meta>`` tag with
  ``name="pypi:repository-version"`` and a ``content`` attribute set to
  a supported repository version (``1.1`` or ``1.2``).
* **JSON** (``application/vnd.pypi.simple.v1+json``): the API v1 JSON index
  for the package.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the artifact must:

* Have been preceded by a recognized request to the simple index URL.
* For HTML responses: contain a ``pypi:repository-version`` meta tag with
  version ``1.1`` or ``1.2``.
* For JSON responses: have content type ``application/vnd.pypi.simple.v1+json``.

Rejection reasons
-----------------

The HTML response is rejected if:

* The ``pypi:repository-version`` meta tag is present but contains an
  unsupported version number.

Extracted metadata
------------------

The following pieces of metadata are extracted by the PyPI simple index
inspector:

.. table:: PyPI simple index inspector metadata (HTML)
   :widths: auto

   ============  ====  ======================================================
   Field         Used  Data source
   ============  ====  ======================================================
   type          Yes   ``text/html``
   name          Yes   ``Simple index for '<package-name>'``
   version
   description   Yes   ``PyPI repository index HTML file for package '<name>'``
   vendor        Yes   Repository hostname
   author        Yes   Repository hostname
   ============  ====  ======================================================

.. table:: PyPI simple index inspector metadata (JSON)
   :widths: auto

   ============  ====  ======================================================
   Field         Used  Data source
   ============  ====  ======================================================
   type          Yes   ``application/json``
   name          Yes   ``JSON index for '<package-name>'``
   version       Yes   ``v1+json``
   description   Yes   ``PyPI repository index JSON file for package '<name>'``
   vendor        Yes   Repository hostname
   author        Yes   Repository hostname
   ============  ====  ======================================================
