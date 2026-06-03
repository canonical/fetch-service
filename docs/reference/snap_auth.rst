.. _ref_snap_auth:

.. meta::
    :description: Reference for the snap authentication inspectors which verify authentication artifacts for the Snap store.

The snapd authentication inspectors
=====================================

Snap store device authentication is a multi-step handshake protocol that
establishes a device identity and obtains Macaroon-based session tokens.
Clients acquire a nonce, obtain a request ID, and exchange those for an
authentication token that is used in subsequent store API calls.

The snapd authentication inspectors examine responses from the Snap store's
device and user authentication endpoints.

There are three inspectors in this group, each covering a specific endpoint:

* ``snap.auth-nonce`` — the nonce endpoint used to initiate device
  authentication.
* ``snap.auth-request-id`` — the request ID endpoint used during device
  serial assertion acquisition.
* ``snap.auth-sessions`` — the sessions endpoint used to acquire a
  Macaroon-based authentication token.

Inspector IDs
-------------

* ``snap.auth-nonce``
* ``snap.auth-request-id``
* ``snap.auth-sessions``

Internal state
--------------

None.

Request verification
--------------------

Each inspector accepts HTTPS requests to the following fixed URLs:

.. list-table::
   :header-rows: 1
   :widths: auto

   * - Inspector
     - URL
   * - ``snap.auth-nonce``
     - ``https://api.snapcraft.io:443/api/v1/snaps/auth/nonces``
   * - ``snap.auth-request-id``
     - ``https://api.snapcraft.io:443/api/v1/snaps/auth/request-id``
   * - ``snap.auth-sessions``
     - ``https://api.snapcraft.io:443/api/v1/snaps/auth/sessions``

File format
-----------

Each inspector expects a JSON response with content type
``application/json`` containing a single field:

* ``snap.auth-nonce``: ``{"nonce": "<value>"}``
* ``snap.auth-request-id``: ``{"request-id": "<value>"}``
* ``snap.auth-sessions``: ``{"macaroon": "<value>"}``

The field value must be non-empty.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the response must:

* Have content type ``application/json``.
* Be a JSON object with exactly the expected field containing a non-empty
  value.
* Have been preceded by a recognized request to the corresponding endpoint.

Rejection reasons
-----------------

The response is not recognized (and therefore not approved) if:

* The content type is not ``application/json``.
* The JSON object does not contain the expected field or the value is empty.
* No prior request to the corresponding URL was recognized.

Extracted metadata
------------------

The following pieces of metadata are extracted by the snapd authentication
inspectors:

.. table:: Snap authentication inspector metadata
   :widths: auto

   ============  ====  =====================================================
   Field         Used  Data source
   ============  ====  =====================================================
   type          Yes   ``application/x.canonical.snapd-auth-nonce``
                       (snap.auth-nonce),
                       ``application/x.canonical.snapd-auth-request-id``
                       (snap.auth-request-id),
                       ``application/x.canonical.snapd-auth-sessions``
                       (snap.auth-sessions)
   name          Yes   ``Authentication nonce`` (snap.auth-nonce),
                       ``Device authentication request ID``
                       (snap.auth-request-id),
                       ``Session authentication`` (snap.auth-sessions)
   description   Yes   ``Snapd authentication nonce`` (snap.auth-nonce),
                       ``Snapd device authentication request ID``
                       (snap.auth-request-id),
                       ``Snapd session authentication`` (snap.auth-sessions)
   version
   vendor
   author
   ============  ====  =====================================================
