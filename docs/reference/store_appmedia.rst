.. _ref_store_appmedia:

.. meta::
    :description: Reference for the store app media inspector which verifies application media images from the Craft store.

The store app media inspector
==============================

App media files are image assets, such as icons and screenshots, associated
with packages published in the Craft store. They are served as PNG images
and are used by store clients to display package listings.

The store app media inspector examines media files (such as icons and
screenshots) served by the Craft store.

Inspector ID
------------

``store.appmedia``

Internal state
--------------

None.

Request verification
--------------------

The store app media inspector accepts HTTP requests to the URLs
configured in the ``store.urls`` entry in ``inspectors.yaml``.

File format
-----------

The inspector currently examines PNG image files (``image/png``).

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

To be approved, the artifact must:

* Have been preceded by a recognized request to a store app media URL.
* Be a valid PNG image file.

Rejection reasons
-----------------

PNG files not from a recognized store app media URL are not approved
(they receive an unknown verdict instead).

Extracted metadata
------------------

The following pieces of metadata are extracted by the store app media
inspector:

.. table:: Store app media inspector metadata
   :widths: auto

   ============  ====  =============================================
   Field         Used  Data source
   ============  ====  =============================================
   type          Yes   ``application/x.canonical.store.appmedia-png``
   name          Yes   ``Image file``
   description   Yes   ``Store media file in PNG format``
   version
   vendor
   author
   ============  ====  =============================================
