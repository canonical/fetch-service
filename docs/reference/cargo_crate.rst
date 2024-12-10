The Cargo crate inspector
=========================

The Cargo crate inspector examines and extracts metadata from Rust crates, as
published in a Cargo registry.

Inspector ID
------------

``cargo.crate``

Internal state
--------------

None

Request verification
--------------------

The current implementation allows downloads from
``https://static.crates.io:443``. This will be changed in the future to match
an internal repository of crates.

During the request the inspector records requested crate name and version.

File format
-----------

To be considered a crate file, an artifact must meet the following criteria:

* It must be a gzip archive
* The archive must contain a root directory called ``<name>-<version>``, where
  ``<name>`` and ``<version>`` are the requested crate name and version
  recorded during the request inspection
* This root directory must contain a file called ``Cargo.toml``, which must
  be a valid TOML file.

Acceptance criteria
-------------------

In order be approved, the downloaded crate file must also comply to
the following requirements:

* The extracted ``Cargo.toml`` must contain the crate's name and version
* The name and version extracted from the ``Cargo.toml`` file must match the
  requested crate's name and version.

Rejection reasons
-----------------

A crate file can be rejected for the following reasons, in addition to
internal or environment errors:

* The crate file is corrupted or has missing elements
* The crate's extracted name and version don't match the request.

Extracted metadata
------------------

The following pieces of metadata are extracted by the crate inspector:

.. table:: Crate inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   name          Yes   ``Cargo.toml`` field ``package.name``
   version       Yes   ``Cargo.toml`` field ``package.version``
   description   Yes   ``Cargo.toml`` field ``package.description``
   author        Yes   ``Cargo.toml`` field ``package.authors``
   license       Yes   ``Cargo.toml`` field ``package.license``
   ============  ====  ============================================
