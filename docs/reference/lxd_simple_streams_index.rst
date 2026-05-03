.. _ref_lxd_simple_streams_index:

.. meta::
    :description: Reference for the LXD Simple Streams index inspector which verifies the top-level Simple Streams index from the LXD image server.

The LXD Simple Streams index inspector
=======================================

Simple Streams is a format for publishing and discovering Ubuntu cloud
images and associated metadata. The Simple Streams index file is the entry
point for image discovery and lists the available streams with the paths to
their corresponding download files.

The LXD Simple Streams index inspector examines the Simple Streams index
JSON file served by the Ubuntu cloud images server.

Inspector ID
------------

``lxd.simple-streams.index``

Internal state
--------------

None.

Request verification
--------------------

The LXD Simple Streams index inspector accepts requests to
``http://cloud-images.ubuntu.com`` and ``https://cloud-images.ubuntu.com:443``
with paths matching: ``/<stream>/streams/v1/index.json``.

File format
-----------

The index JSON file must be a JSON object containing:

* ``format``: the string ``"index:1.0"``.
* ``index``: a map of entries, each with:

  * ``format``: the string ``"products:1.0"``.
  * ``datatype``: the string ``"image-downloads"``.
  * ``path``: a non-empty path string.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the artifact must:

* Have content type ``application/json``.
* Parse as a valid JSON file with ``format`` set to ``"index:1.0"``.
* All entries in the ``index`` map (if any) must have ``format`` set to
  ``"products:1.0"`` and ``datatype`` set to ``"image-downloads"``.

Rejection reasons
-----------------

The artifact is rejected if:

* Any index entry has a ``format`` other than ``"products:1.0"``.
* Any index entry has a ``datatype`` other than ``"image-downloads"``.

Extracted metadata
------------------

The following pieces of metadata are extracted by the LXD Simple Streams
index inspector:

.. table:: LXD Simple Streams index inspector metadata
   :widths: auto

   ============  ====  ================================================
   Field         Used  Data source
   ============  ====  ================================================
   type          Yes   ``application/x.canonical.simplestreams-index``
   name          Yes   ``Simple Streams Index``
   description   Yes   ``Simple Streams Index for <stream>``
   version
   vendor
   author
   ============  ====  ================================================
