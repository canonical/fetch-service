.. _ref_snap_sections:

.. meta::
    :description: Reference for the snap sections inspector which verifies the snap sections API response from the Snap store.

The snap sections inspector
============================

The snap sections response is part of the Snap store's v1 REST API. It
provides the list of editorial sections that organise snaps in the store.

The snap sections inspector examines responses from the Snap store's store
sections list endpoint.

Inspector ID
------------

``snap.sections``

Internal state
--------------

None.

Request verification
--------------------

The snap sections inspector accepts HTTPS requests to:
``https://api.snapcraft.io:443/api/v1/snaps/sections``

File format
-----------

The inspector expects a JSON response with an ``_embedded`` object
containing a non-empty ``clickindex:sections`` list, where each entry has
a ``name`` field.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the response must:

* Have content type ``application/json``.
* Have been preceded by a recognized request to the snap sections URL.
* Parse as a valid JSON object with a non-empty sections list.

Rejection reasons
-----------------

The response is not recognized (and therefore not approved) if:

* The content type is not ``application/json``.
* No prior recognized request to the snap sections URL was made.
* The JSON cannot be parsed.
* The ``_embedded.clickindex:sections`` list is missing or empty.

Extracted metadata
------------------

The following pieces of metadata are extracted by the snap sections
inspector:

.. table:: Snap sections inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.canonical.snap-sections``
   name          Yes   ``Snap sections list``
   description   Yes   ``List of Snap Store sections``
   version
   vendor
   author
   ============  ====  ============================================
