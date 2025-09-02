The resolve_revisions API index inspector
=========================================

The resolve_revisions API inspector examines requests to the Craft store's
resolve_revisions endpoint.

Inspector ID
------------

``store.resolve-api``

Internal state
--------------

None

Request verification
--------------------

Requests to the ``/v2/revisions/resolve`` path in URLs matching entries
configured under the ``store.urls`` key in the ``inspectors.yaml``
configuration file are considered valid.

Acceptance criteria
-------------------

To be considered valid, the response for resolve API requests:

* Must contain valid JSON-encoded data.
* Must contain non-empty ``craft-results`` and ``package-results`` keys
  with at least one element containing non-empty ``namespace`` string and
  ``status`` set to ``ok`` or ``error``.

Rejection reasons
-----------------

* The namespace is invalid. Valid namespaces are ``bin``, ``charm``,
  ``rock`` and ``snap``.

Extracted metadata
------------------

The following pieces of metadata are extracted by the index inspector:

.. table:: config.json metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   name          Yes   "Store protocol response"
   description   Yes   "Store response for resolve_revisions request"
   vendor
   author
   author-email
   architecture
   license
   copyright
   ============  ====  ============================================
