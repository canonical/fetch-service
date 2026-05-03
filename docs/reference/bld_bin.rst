.. _ref_bld_bin:

.. meta::
    :description: Reference for the build binary package inspector which verifies pre-compiled binary artifacts published in the Craft store.

The build binary package inspector
===================================

Build binary packages are pre-compiled binary artifacts published through
the Craft store's ``bins`` channel. They are distributed as XZ-compressed
tar archives and supply pre-built binaries to Craft build environments.

The build binary package inspector examines binary packages as published
in the Craft store's ``bins`` channel.

Inspector ID
------------

``bld.bin``

Internal state
--------------

None.

Request verification
--------------------

The build binary package inspector accepts HTTP requests to the repositories
configured in the ``bldbin.urls`` entry in ``inspectors.yaml``.

File format
-----------

The build binary package inspector expects the artifact to be an
XZ-compressed tar archive containing a ``./metadata.yaml`` file with the
following required fields:

* ``name``: the package name.
* ``version``: the package version.
* ``base``: the base image the package runs on.
* ``architecture``: the target machine architecture.

Configuration options
---------------------

This inspector is configured under the ``bldbin`` key in ``inspectors.yaml``.

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

* Be an XZ-compressed tar archive.
* Contain a ``./metadata.yaml`` file with non-empty ``name``, ``version``,
  ``base``, and ``architecture`` fields.

Rejection reasons
-----------------

The artifact is rejected if:

* It is not a valid XZ-compressed tar archive.
* The ``./metadata.yaml`` file is missing or cannot be decoded.
* Any of the required fields (``name``, ``version``, ``base``, or
  ``architecture``) are empty.

Extracted metadata
------------------

The following pieces of metadata are extracted by the build binary package
inspector:

.. table:: Build binary package inspector metadata
   :widths: auto

   ==============  ====  ============================================
   Field           Used  Data source
   ==============  ====  ============================================
   type            Yes   ``application/x.canonical.bld-bin-package``
   name            Yes   ``metadata.yaml`` field ``name``
   version         Yes   ``metadata.yaml`` field ``version``
   description     Yes   ``metadata.yaml`` field ``summary``
   architecture    Yes   ``metadata.yaml`` field ``architecture``
   license         Yes   ``metadata.yaml`` field ``license``
   vendor          Yes   ``metadata.yaml`` field ``contact``
   author
   store-revision  Yes   Revision from :doc:`store.info-api <store_info_api>`
   content-id      Yes   Package ID from :doc:`store.info-api <store_info_api>`
   ==============  ====  ============================================
