The Chisel release inspector
============================


The Chisel release inspector verifies the `chisel-releases repository`_'s
tarball download request and response artifact.

`Chisel`_ downloads the tarball via a GET request. The inspector monitors this
request and currently only examines the URL and the gzip compressed tarball. It
checks if the artifact tarball contains the appropriate files.


Inspector ID
------------

``chisel.release``


Internal state
--------------

None.


Request verification
--------------------

The Chisel release inspector accepts HTTPS GET requests to the
`chisel-releases repository`_.

The URL is expected to match any URL patterns specified in the inspectors
configuration. Specify URL patterns for this inspector like the following:

.. code-block:: yaml

  # inspectors.yaml
  chisel:
    urls:
      - https://codeload.github.com:443/canonical/chisel-releases/**
      - ...


File format
-----------

The Chisel release inspector ensures the downloaded file:

* Is a gzip compressed tar file.
* Contains a valid ``chisel.yaml`` file inside.


Acceptance criteria
-------------------

To be approved, the artifact examined by this inspector must comply to the
following rules:

* It must have previously described file format.

* The ``chisel.yaml`` file must have the following properties:

  * a non-empty ``format`` field.
  * a non-empty ``archives`` field.
  * a non-empty ``components`` field for each ``archive``.
  * a non-empty ``suites`` field for each ``archive``.
  * a non-empty ``public-keys`` field for each ``archive``.
  * a non-empty ``public-keys`` field.
  * a non-empty ``id`` field for each ``public-keys`` entry.
  * a non-empty and valid ``armor`` field for each ``public-keys`` entry.

* At least one of the public keys defined in ``public-keys`` must match any of
  the repository public keys in ``apt`` configuration.


Rejection reasons
-----------------

The artifact is rejected if:

* It contains the ``chisel.yaml`` file , but the file is not valid
  according to the acceptance criteria described above.

Other artifacts, if not approved, are ignored by the inspector.


Extracted metadata
------------------

The following pieces of metadata are extracted by the Chisel release inspector:

.. table:: Chisel release inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.canonical.chisel.release``
   name          Yes   ``chisel-release``
   version       Yes   ``chisel.yaml`` field ``format`` e.g. ``v1``
   description   Yes   ``Chisel release file for <release>``
   vendor        Yes   ``Canonical``
   author
   author-email
   architecture
   license
   copyright
   ============  ====  ============================================


.. Links

.. _Chisel: https://github.com/canonical/chisel
.. _chisel-releases repository: https://github.com/canonical/chisel-releases
