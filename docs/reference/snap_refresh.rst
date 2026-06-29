.. _ref_snap_refresh:

.. meta::
    :description: Reference for the snap refresh inspector which verifies snap refresh API responses from the Snap store.

The snap refresh inspector
===========================

The snap refresh response is part of the Snap store's v2 REST API. It is
used by ``snapd`` to check for and retrieve updates for installed snaps,
returning the effective channel and resolved revision for each snap.

The snap refresh inspector examines responses from the Snap store's snap
refresh API endpoint.

Inspector ID
------------

``snap.refresh``

Internal state
--------------

None.

Request verification
--------------------

The snap refresh inspector accepts HTTPS requests to:
``https://api.snapcraft.io:443/v2/snaps/refresh``

File format
-----------

The inspector expects a JSON response with a ``results`` list where the
first entry contains non-empty ``effective-channel``, ``name``, and
``snap-id`` fields.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the response must:

* Have content type ``application/json``.
* Parse as a valid JSON object.
* Contain a non-empty ``results`` list where the first entry has
  non-empty ``effective-channel``, ``name``, and ``snap-id`` fields.

Rejection reasons
-----------------

The response is not recognized (and therefore not approved) if:

* The content type is not ``application/json``.
* The JSON cannot be parsed.
* The ``results`` list is empty or missing required fields.

Extracted metadata
------------------

The following pieces of metadata are extracted by the snap refresh inspector:

.. table:: Snap refresh inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.canonical.snap-refresh``
   name          Yes   ``Store protocol response``
   description   Yes   ``Snap store response for refresh request``
   version
   vendor
   author
   content-id    Yes   ``<effective-channel>:<revision>`` for single-result responses only
   ============  ====  ============================================
