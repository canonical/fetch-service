.. _ref_snap_refresh:

.. meta::
    :description: Reference for the snap refresh inspector which verifies snap refresh API responses from the Snap store.

The snap refresh inspector
===========================

The snap refresh response is part of the Snap store's v2 REST API. It is
used by ``snapd`` to check for and retrieve updates for installed snaps,
returning the effective channel and resolved revision for each snap.

The snap refresh inspector examines requests and responses for the Snap
store's refresh API endpoint. It also keeps per-session correlation state so
that it can annotate matching snap package downloads with the requested and
effective channels learned from the refresh exchange.

Inspector ID
------------

``snap.refresh``

Internal state
--------------

The inspector keeps an in-memory mapping from ``<snap-id>:<revision>`` to the
requested channel and the resolved refresh response fields for the current
session.

Request verification
--------------------

The snap refresh inspector accepts HTTPS requests to:
``https://api.snapcraft.io:443/v2/snaps/refresh``

When present, the request body is parsed and the first non-empty
``actions[].channel`` value is recorded as the ``requested-channel`` request
annotation.

If a previous refresh response has already resolved a specific
snap ID and revision pair, the inspector also recognizes a matching snap
download request and records the correlated refresh annotations on that
request.

File format
-----------

The inspector expects a JSON response with a ``results`` list where the
first entry contains non-empty ``effective-channel``, ``name``, and
``snap-id`` fields.

For correlated snap downloads, the inspector recognizes SquashFS snap
packages as a secondary artifact type. In that case it does not approve the
snap package itself; it only attaches refresh correlation annotations.

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

For matching snap package downloads, the inspector sets an ``Unknown``
response opinion with annotations if the request URL identifies a
``<snap-id>:<revision>`` pair that was previously resolved by a refresh
response in the same session.

Rejection reasons
-----------------

The response is not recognized (and therefore not approved) if:

* The content type is not ``application/json``.
* The JSON cannot be parsed.
* The ``results`` list is empty or missing required fields.

Matching snap package downloads are not rejected by this inspector. They are
left to the primary snap package inspector for validation and approval.

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
   content-id    Yes   ``<effective-channel>:<revision>``
   ============  ====  ============================================

Annotations
-----------

For matching snap package downloads, the inspector adds the following response
annotations with an ``Unknown`` opinion:

* ``snap-id``
* ``revision``
* ``requested-channel``
* ``effective-channel``
