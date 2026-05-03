.. _ref_snap_assertion:

.. meta::
    :description: Reference for the snap assertion inspector which verifies Ubuntu assertion files from the Snap store.

The snap assertion inspector
=============================

Assertion files are cryptographically signed documents used in the Snap
platform to certify properties of snaps, snap revisions, publishers, and
devices. They establish the trust chain that lets ``snapd`` verify the
authenticity of installed snaps.

The snap assertion inspector examines snap assertion files downloaded from
the Snap store. Assertions are signed documents that certify properties of
snaps, publishers, and devices.

Inspector ID
------------

``snap.assertion``

Internal state
--------------

None.

Request verification
--------------------

The snap assertion inspector accepts HTTPS requests to
``https://api.snapcraft.io:443`` for the following assertion types:

* ``/v2/assertions/snap-revision/...``
* ``/v2/assertions/snap-declaration/...``
* ``/v2/assertions/account/...``
* ``/v2/assertions/account-key/...``
* ``/api/v1/snaps/auth/devices/`` (serial assertions)

File format
-----------

The snap assertion inspector expects the response to:

* Have content type ``application/x.ubuntu.assertion``.
* Contain a valid Ubuntu assertion document.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the assertion must:

* Be a parseable Ubuntu assertion.
* Have a valid cryptographic signature.

Rejection reasons
-----------------

The assertion is rejected if:

* The assertion document cannot be parsed.
* The cryptographic signature is invalid.

Extracted metadata
------------------

The following pieces of metadata are extracted by the snap assertion
inspector. The MIME type depends on the assertion type:

.. table:: Snap assertion inspector metadata
   :widths: auto

   ============  ====  ===========================================================
   Field         Used  Data source
   ============  ====  ===========================================================
   type          Yes   ``application/x.ubuntu.assertion.snap-revision``
                       (snap-revision),
                       ``application/x.ubuntu.assertion.snap-declaration``
                       (snap-declaration),
                       ``application/x.ubuntu.assertion.account``
                       (account),
                       ``application/x.ubuntu.assertion.account-key``
                       (account-key),
                       ``application/x.ubuntu.assertion.serial``
                       (serial)
   name          Yes   ``assertion``
   version       Yes   assertion header ``revision``
   vendor        Yes   assertion header ``authority-id``
   description   Yes   ``<type> assertion file``
   author        Yes   assertion header ``authority-id``
   ============  ====  ===========================================================
