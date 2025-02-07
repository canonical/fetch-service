The Chisel release inspector
============================


The Chisel release inspector verifies the `chisel-releases repository`_'s
tarball download request and response artifact.

`Chisel`_ downloads the tarball via a GET request to
https://codeload.github.com/canonical/chisel-releases/. The inspector monitors
this request and currently only examines the gzip compressed tarball and checks
if it contains the appropriate files.


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

The URL is expected to match the regular expression
``https://codeload.github.com:443/canonical/chisel-releases/tar.gz/refs/heads/([a-z](?:-?[a-z0-9]){2,}-[0-9]+(?:\.?[0-9])+)``.


File format
-----------

The Chisel release inspector ensures the downloaded file:

* Is a gzip compressed tar file.
* Contains a valid ``chisel-releases-<branch>/chisel.yaml`` file inside.
* Contains a ``chisel-releases-<branch>/slices/`` directory inside.

Here, ``<branch>`` is the slug matched from the request described above.


Acceptance criteria
-------------------

To be approved, the artifact examined by this inspector must comply to the
following rules:

* It must have previously described file format.
* The ``chisel.yaml`` file must have the following properties:
	- a non-empty ``format`` field.
	- a non-empty ``archives`` field.
	- a non-empty ``components`` field for each ``archive``.
	- a non-empty ``suites`` field for each ``archive``.


Rejection reasons
-----------------

The artifact is rejected if:

* It contains the ``chisel.yaml`` file and ``slices/`` directory, but the
  ``chisel.yaml`` file is not valid according to the acceptance criteria described
  above.

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
   name          Yes   ``<branch>`` e.g. ``ubuntu-22.04``
   version       Yes   ``chisel.yaml`` field ``format`` e.g. ``v1``
   description
   vendor
   author
   author-email
   architecture
   license
   copyright
   ============  ====  ============================================


.. Links

.. _Chisel: https://github.com/canonical/chisel
.. _chisel-releases repository: https://github.com/canonical/chisel-releases
