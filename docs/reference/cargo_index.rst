.. _ref_cargo_index:

.. meta::
    :description: Reference for the Cargo index inspector which verifies Cargo registry index files containing crate metadata.

The Cargo index inspector
=========================

The Cargo registry index stores metadata about the crates hosted in a Cargo
registry. Package managers use it to resolve crate names and versions without
downloading full crate archives.

The Cargo index inspector examines requests for the index portion of a Cargo
registry - the part containing metadata about the registry itself and the
crates hosted there.

Inspector ID
------------

``cargo.index``

Internal state
--------------

None.

Request verification
--------------------

The Cargo index inspector recognizes requests to ``https://index.crates.io:443``.

Two types of requests are allowed: requests for ``/config.json``, which is the
JSON file containing metadata about the registry, and requests for the index
of a specific crate. These requests have the form `/ab/cd/abcd`, where
``abcd`` is the crate's name.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

The acceptance criteria depends on the type of request:

* The response for ``/config.json`` requests must be a valid JSON file with
  ``dl`` and ``api`` keys.
* The response for individual crate indices must be a newline-delimited JSON
  (ndjson) file, where the first line is a valid JSON object containing a
  ``name`` that matches the requested crate.

Rejection reasons
-----------------

* The ``/config.json`` response artifact will be rejected if it's not a valid
  JSON file.
* The artifact for the index of a specific crate will be rejected if it's not
  a valid ndjson file, or if the first line of the file does not have a
  ``name`` key that matches the requested crate's name.

Extracted metadata
------------------

The following pieces of metadata are extracted by the index inspector:

.. table:: config.json metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/json``
   name          Yes   ``config.json for Cargo package index``
   version
   description   Yes   ``config.json for Cargo package index``
   vendor        Yes   ``config.json`` key ``api``
   author        Yes   ``config.json`` key ``api``
   ============  ====  ============================================

.. table:: crate index metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x-ndjson``
   name          Yes   ``name`` key from first line in ndjson
   version
   description   Yes   ``Cargo package index for crate "<name>"``
   vendor
   author
   ============  ====  ============================================
