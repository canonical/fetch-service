.. _ref_store_transforms_api:

.. meta::
    :description: Reference for the store transforms API inspector which verifies workspace transform responses from the Craft store.

The store transforms API inspector
====================================

The Craft store workspace transforms API is part of the Craft store's build
toolchain protocol. It is a JSON-encoded response that lists the binary
transforms available for a given workspace.

The store transforms API inspector examines responses from the Craft
store's workspace transforms endpoint.

Inspector ID
------------

``store.transforms-api``

Internal state
--------------

None.

Request verification
--------------------

Requests to URLs configured under the ``store.urls`` key in
``inspectors.yaml`` that match the store transforms API path pattern are
considered valid.

File format
-----------

The inspector expects a JSON response containing:

* A non-empty ``workspace-id`` string.
* A ``transforms`` list (which may be empty), where each entry contains a
  ``package`` object with a ``type`` field.

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
* Parse as a valid JSON object with a non-empty ``workspace-id`` and a
  present ``transforms`` field (which may be an empty list).
* All transform entries must have ``package.type`` set to ``"bin"``.

Rejection reasons
-----------------

The response is rejected if:

* The content type is not ``application/json``.
* The JSON cannot be parsed.
* The ``workspace-id`` field is missing or empty, or the ``transforms``
  field is absent from the JSON.
* Any transform entry has a ``package.type`` other than ``"bin"``.

Extracted metadata
------------------

The following pieces of metadata are extracted by the store transforms API
inspector:

.. table:: Store transforms API inspector metadata
   :widths: auto

   ============  ====  ==============================================
   Field         Used  Data source
   ============  ====  ==============================================
   type          Yes   ``application/x.canonical.store.transforms-api``
   name          Yes   ``Store protocol response``
   description   Yes   ``Store response for workspace transforms request``
   version
   vendor
   author
   ============  ====  ==============================================
