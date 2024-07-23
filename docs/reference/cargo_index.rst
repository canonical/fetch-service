The Cargo index inspector
=========================

The Cargo index inspector examines requests for the index portion of a Cargo
registry - the part containing metadata about the registry itself and the
crates hosted there.

Inspector ID
------------

``cargo.index``

Internal state
--------------

None

Request verification
--------------------

The current implementation allows downloads from
``https://index.crates.io:443``. This will be changed in the future to match
an internal repository of crates.

Two types of requests are allowed: requests for ``/config.json``, which is the
JSON file containing metadata about the registry, and requests for the index
of a specific crate. These requests have the form `/ab/cd/abcd`, where
``abcd`` is the crate's name.

Acceptance criteria
-------------------

The acceptance criteria depends on the type of request:

* The response for ``/config.json`` requests must be a valid JSON file with
  ``dl`` and ``api`` keys.
* The response for individual crate indices must be a newline-delimited JSON
  (ndjson) file, where each line is a valid JSON object containing a ``name``
  that matches the requested crate.

Rejection reasons
-----------------

* The ``/config.json`` response artefact will be rejected if it's not a valid
  JSON file.
* The artefact for the index of a specific crate will be rejected if it's not
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
   name          Yes   "config.json for Cargo package index"
   description   Yes   "config.json for Cargo package index"
   vendor        Yes   ``config.json`` key ``api``
   vendor        Yes   ``config.json`` key ``api``
   ============  ====  ============================================

.. table:: crate index metadata

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   name          Yes   ``name`` key from first line in ndjson
   description   Yes   "Cargo package index for crate <name>"
   ============  ====  ============================================
