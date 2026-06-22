.. _ref_lxd_rootfs:

.. meta::
    :description: Reference for the LXD rootfs inspector which verifies LXD container image tarballs.

The LXD rootfs inspector
========================

An LXD rootfs image is a gzip-compressed tar archive containing a minimal
Linux filesystem and image metadata. These images are derived from Ubuntu
cloud images and are used by LXD to create containers.

The LXD rootfs inspector examines LXD rootfs image tarballs downloaded from
the Ubuntu cloud images server.

Inspector ID
------------

``lxd.rootfs``

Internal state
--------------

None.

Request verification
--------------------

The LXD rootfs inspector accepts requests to
``http://cloud-images.ubuntu.com`` and ``https://cloud-images.ubuntu.com:443``
with paths matching:
``/buildd/(daily|releases)/<series>/<datestamp>/<image-name>.tar.gz``.

File format
-----------

The LXD rootfs inspector expects the artifact to be a gzip-compressed tar
archive containing:

* A ``metadata.yaml`` file (JSON-encoded) with the fields ``architecture``,
  ``creation_date``, and ``properties`` (containing ``description``,
  ``os``, ``series``, and ``architecture``).
* At least one entry under the ``rootfs/`` directory.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the artifact must:

* Parse as a gzip-compressed tar archive.
* Contain both a valid ``metadata.yaml`` and a ``rootfs/`` directory.
* Be listed in a previously downloaded and approved Simple Streams download
  file (verified by the :doc:`lxd.simple-streams.download <lxd_simple_streams_download>` inspector).

Rejection reasons
-----------------

The artifact is not recognized (and therefore not approved) if:

* It cannot be decompressed or parsed as a tar archive.
* The ``metadata.yaml`` file is missing or cannot be decoded.
* The ``architecture`` or ``properties.os`` fields are empty.
* No ``rootfs/`` directory entry is found in the archive.

The artifact is rejected if:

* No matching product item was recorded by the
  :doc:`lxd.simple-streams.download <lxd_simple_streams_download>` inspector.

Extracted metadata
------------------

The following pieces of metadata are extracted by the LXD rootfs inspector:

.. table:: LXD rootfs inspector metadata
   :widths: auto

   ============  ====  =============================================
   Field         Used  Data source
   ============  ====  =============================================
   type          Yes   ``application/x.canonical.lxd-rootfs``
   name          Yes   ``LXD rootfs image``
   version       Yes   ``creation_date`` field (Unix timestamp)
   description   Yes   ``properties.description`` from metadata
   vendor
   author
   architecture  Yes   ``properties.architecture`` from metadata
   ============  ====  =============================================
