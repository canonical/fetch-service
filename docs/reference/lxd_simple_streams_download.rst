.. _ref_lxd_simple_streams_download:

.. meta::
    :description: Reference for the LXD Simple Streams download inspector which verifies product download index files from the LXD image server.

The LXD Simple Streams download inspector
==========================================

Simple Streams is a format for publishing and discovering Ubuntu cloud
images and associated metadata. A Simple Streams download file lists the
downloadable products for an image stream, including their paths and SHA256
checksums.

The LXD Simple Streams download inspector examines LXD Simple Streams
product download JSON files and the product item tarballs they reference.

Inspector ID
------------

``lxd.simple-streams.download``

Internal state
--------------

* A map of product item file paths to their expected SHA256 digests,
  populated when a download JSON file is approved.

Request verification
--------------------

The LXD Simple Streams download inspector accepts requests to
``http://cloud-images.ubuntu.com`` and ``https://cloud-images.ubuntu.com:443``
with paths matching:

* Download JSON: ``/<stream>/streams/v1/<name>:download.json``
* Product items: ``/buildd/(daily|releases)/<series>/<datestamp>/<name>.tar.gz``

File format
-----------

The download JSON file must be a JSON object containing:

* ``format``: the string ``"products:1.0"``.
* ``datatype``: a non-empty string.
* ``updated``: a non-empty timestamp string.
* ``products``: a map of product entries.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the download JSON artifact must contain a valid
``products:1.0`` JSON response.

Product item tarballs (``.tar.gz`` files) are verified against the SHA256
digests recorded from the corresponding download JSON file. A matching item
is marked as unknown (for further inspection by the
:doc:`lxd.rootfs <lxd_rootfs>` inspector).

Rejection reasons
-----------------

Product item tarballs are rejected if:

* The SHA256 digest of the downloaded file does not match the value
  recorded in the Simple Streams download JSON.
* No SHA256 digest was recorded for the item path.

Extracted metadata
------------------

The following pieces of metadata are extracted by the LXD Simple Streams
download inspector:

.. table:: LXD Simple Streams download inspector metadata
   :widths: auto

   ============  ====  ===================================================
   Field         Used  Data source
   ============  ====  ===================================================
   type          Yes   ``application/x.canonical.simplestreams-products``
   name          Yes   ``Simple Streams Download``
   description   Yes   ``Simple Streams Download for <content-id>``
   version
   vendor
   author
   ============  ====  ===================================================
