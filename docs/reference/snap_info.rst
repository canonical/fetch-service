.. _ref_snap_info:

.. meta::
    :description: Reference for the snap info API inspector which verifies snap info API responses from the Snap store.

The snap info inspector
========================

The snap info response is part of the Snap store's v2 REST API. It
describes a snap package and its available channels, providing the snap's
name, ID, and channel map.

The snap info inspector examines responses from the Snap store's snap info
API endpoint.

Inspector ID
------------

``snap.info``

Internal state
--------------

None.

Request verification
--------------------

The snap info inspector accepts HTTPS requests to:
``https://api.snapcraft.io:443/v2/snaps/info/<snap-name>``

File format
-----------

The inspector expects a JSON response with:

* A non-empty ``channel-map`` list where the first entry contains a
  non-empty ``version`` field.
* A non-empty ``name`` field.
* A non-empty ``snap-id`` field.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the response must:

* Have content type ``application/json``.
* Parse as a valid JSON object.
* Contain a non-empty ``channel-map`` list, ``name``, and ``snap-id``.

Rejection reasons
-----------------

The response is not recognized (and therefore not approved) if:

* The content type is not ``application/json``.
* The JSON cannot be parsed.
* The ``channel-map``, ``name``, or ``snap-id`` fields are missing or empty.

Extracted metadata
------------------

The following pieces of metadata are extracted by the snap info inspector:

.. table:: Snap info inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.canonical.snap-info``
   name          Yes   ``Store protocol response``
   description   Yes   ``Snap store response for info request``
   version
   vendor
   author
   ============  ====  ============================================
