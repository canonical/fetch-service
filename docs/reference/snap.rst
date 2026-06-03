.. _ref_snap:

.. meta::
    :description: Reference for the snap package inspector which verifies snap packages downloaded from the Snap store.

The snap package inspector
==========================

A snap is a self-contained application package for Linux, bundled with all
its dependencies as a SquashFS filesystem image. Snaps are installed and
managed by the ``snapd`` daemon and are published through the Snap store.

The snap package inspector examines snap packages downloaded from the
Snap store.

Inspector ID
------------

``snap``

Internal state
--------------

None.

Request verification
--------------------

The snap package inspector accepts HTTPS requests to:

* ``https://api.snapcraft.io:443/api/v1/snaps/download/<snap-id>_<revision>.snap``
* ``https://<host>.snapcraftcontent.com:443/.../<snap-id>_<revision>.snap``

File format
-----------

A snap package is a SquashFS filesystem image. The inspector expects the
snap to contain:

* A ``meta/snap.yaml`` file with at least ``name``, ``version``, ``summary``,
  and ``architectures`` fields.

Configuration options
---------------------

This inspector is configured under the ``snap`` key in ``inspectors.yaml``.

.. list-table::
   :widths: auto
   :header-rows: 1

   * - Option
     - Description
   * - ``snap-declaration``
     - List of snap-declaration attribute filters. Each filter specifies
       a ``name`` (the attribute name in the ``snap-declaration``
       assertion) and a ``value`` (list of allowed values). Only snaps
       whose snap-declaration assertion matches all specified filters are
       approved.

Acceptance criteria
-------------------

To be approved, the snap package must:

* Be a valid SquashFS filesystem image.
* Contain a parseable ``meta/snap.yaml`` file.
* Have a snap-revision assertion available, downloaded directly from the
  Snap store API and verified against the snap's SHA3-384 digest.
* Have a snap-declaration assertion available with a valid cryptographic
  signature.
* Have an account assertion available with a valid cryptographic signature.
* Pass any snap-declaration filter rules configured for the inspector.

Rejection reasons
-----------------

The snap is rejected if:

* The SquashFS image cannot be read.
* The ``meta/snap.yaml`` file is missing or cannot be decoded.
* A snap-declaration or account assertion is not found or has an invalid
  signature.
* The snap-declaration filter check fails (for example, publisher or snap
  name is not in the allowed list).

Extracted metadata
------------------

The following pieces of metadata are extracted by the snap package inspector:

.. table:: Snap package inspector metadata
   :widths: auto

   ==============  ====  =============================================
   Field           Used  Data source
   ==============  ====  =============================================
   type            Yes   ``application/x.canonical.snap-package``
   name            Yes   Snap name from snap-declaration assertion
   version         Yes   ``version`` key in ``meta/snap.yaml``
   description     Yes   ``summary`` key in ``meta/snap.yaml``
   vendor          Yes   Publisher display name from account assertion
   author
   architecture    Yes   ``architectures`` list joined with ``,``
   license         Yes   ``license`` key in ``meta/snap.yaml``
   store-revision  Yes   Snap revision from snap-revision assertion
   content-id      Yes   Snap ID from snap-revision assertion
   ==============  ====  =============================================
