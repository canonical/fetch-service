.. _ref_store_resolve_api:

.. meta::
    :description: Reference for the store resolve API inspector which verifies revision resolution responses from the Craft store.

The resolve_revisions API inspector
=====================================

The Craft store ``resolve_revisions`` API is part of the Craft store's
package resolution protocol. It is a JSON-encoded response that maps
package requests to their resolved revisions and statuses.

The resolve_revisions API inspector examines requests to the Craft store's
``resolve_revisions`` endpoint.

Inspector ID
------------

``store.resolve-api``

Internal state
--------------

None.

Request verification
--------------------

Requests to the ``/v2/revisions/resolve`` path in URLs matching entries
configured under the ``store.urls`` key in the ``inspectors.yaml``
configuration file are considered valid.

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

To be considered valid, the response for resolve API requests:

* Must contain valid JSON-encoded data.
* Must contain at least one of ``craft-results`` or ``package-results`` keys
  as a non-empty list, where at least one element contains a non-empty
  ``namespace`` string and ``status`` set to ``ok`` or ``error``.

Rejection reasons
-----------------

* The namespace is invalid. Valid namespaces are ``bin``, ``charm``,
  ``rock`` and ``snap``.

Extracted metadata
------------------

The following pieces of metadata are extracted by the resolve API inspector:

.. table:: Store resolve API inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.canonical.store.resolve-api``
   name          Yes   ``Store protocol response``
   description   Yes   ``Store response for resolve_revisions request``
   version
   vendor
   author
   author-email
   architecture
   ============  ====  ============================================
