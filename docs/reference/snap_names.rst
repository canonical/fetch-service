.. _ref_snap_names:

.. meta::
    :description: Reference for the snap names inspector which verifies the snap names API response from the Snap store.

The snap names inspector
=========================

The snap names response is part of the Snap store's v1 REST API. It
provides a list of all public snap package names available in the store.

The snap names inspector examines responses from the Snap store's package
names list endpoint.

Inspector ID
------------

``snap.names``

Internal state
--------------

None.

Request verification
--------------------

The snap names inspector accepts HTTPS requests to:
``https://api.snapcraft.io:443/api/v1/snaps/names``

File format
-----------

The inspector expects a JSON response with an ``_embedded`` object
containing a non-empty ``clickindex:package`` list, where each entry
contains at least a ``package-name`` field.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the response must:

* Have content type ``application/json``.
* Have been preceded by a recognized request to the snap names URL.
* Parse as a valid JSON object with a non-empty package list.

Rejection reasons
-----------------

The response is not recognized (and therefore not approved) if:

* The content type is not ``application/json``.
* No prior recognized request to the snap names URL was made.
* The JSON cannot be parsed.
* The ``_embedded.clickindex:package`` list is missing or empty.

Extracted metadata
------------------

The following pieces of metadata are extracted by the snap names inspector:

.. table:: Snap names inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.canonical.snap-names``
   name          Yes   ``Snap names list``
   description   Yes   ``List of Snap package names``
   version
   vendor
   author
   ============  ====  ============================================
