.. _ref_store_info_api:

.. meta::
    :description: Reference for the store info API inspector which verifies package information responses from the Craft store.

The store info API inspector
=============================

The Craft store info API response is part of the Craft store's package
management protocol. It is a JSON-encoded response describing a specific
package revision, including its download URL, integrity digest, and
publisher information.

The store info API inspector examines responses from the Craft store's
package info endpoint.

Inspector ID
------------

``store.info-api``

Internal state
--------------

* A map of package IDs to revision information (SHA3-384 digest, size,
  revision number, and channel), populated when an info API response
  is approved.

Request verification
--------------------

Requests to URLs configured under the ``store.urls`` key in
``inspectors.yaml`` that match the store info API path pattern are
considered valid.

File format
-----------

The inspector expects a JSON response containing:

* A non-empty ``channel-map`` list where the first entry contains a
  non-empty ``revision.download.url``.
* A non-empty ``name`` and ``package-id``.
* A ``metadata.publisher.username`` field.

Configuration options
---------------------

This inspector is configured under the ``store`` key in ``inspectors.yaml``.

.. list-table::
   :widths: auto
   :header-rows: 1

   * - Option
     - Description
   * - ``urls``
     - List of URL glob patterns. Only requests to matching URLs are
       approved for further inspection.

Acceptance criteria
-------------------

To be approved, the response must:

* Have content type ``application/json``.
* Parse as a valid JSON object matching the expected structure above.

Rejection reasons
-----------------

The response is not recognized (and therefore not approved) if:

* The content type is not ``application/json``.
* The JSON cannot be parsed or does not match the expected structure.
* Required fields (``channel-map``, ``name``, ``package-id``, or
  publisher username) are missing.

Extracted metadata
------------------

The following pieces of metadata are extracted by the store info API
inspector:

.. table:: Store info API inspector metadata
   :widths: auto

   ============  ====  =============================================
   Field         Used  Data source
   ============  ====  =============================================
   type          Yes   ``application/x.canonical.store.info-api``
   name          Yes   ``Store protocol response``
   description   Yes   ``Store response for info request``
   version
   vendor
   author
   ============  ====  =============================================
