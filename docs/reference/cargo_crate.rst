.. _ref_cargo_crate:

.. meta::
    :description: Reference for the Cargo crate inspector which verifies Rust crate archives from a Cargo registry.

The Cargo crate inspector
=========================

A Rust crate is the fundamental unit of code distribution in the Rust
language. Crates are published to Cargo registries such as crates.io and
distributed as gzip-compressed tar archives containing source code and a
``Cargo.toml`` manifest.

The Cargo crate inspector examines and extracts metadata from Rust crates, as
published in a Cargo registry.

Inspector ID
------------

``cargo.crate``

Internal state
--------------

None.

Request verification
--------------------

The Cargo crate inspector recognizes requests to
``https://static.crates.io:443/crates/<name>/<version>/download``.
These requests are marked as unknown (unsupported origin) because
``static.crates.io`` is not a configured trusted origin.

During the request the inspector records requested crate name and version.

File format
-----------

To be considered a crate file, an artifact must meet the following criteria:

* It must be a gzip-compressed tar archive
* The archive must contain a root directory called ``<name>-<version>``, where
  ``<name>`` and ``<version>`` are the requested crate name and version
  recorded during the request inspection
* This root directory must contain a file called ``Cargo.toml``, which must
  be a valid TOML file.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the downloaded crate file must also comply to
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
   type          Yes   ``application/x.rust.crate``
   name          Yes   ``Cargo.toml`` field ``package.name``
   version       Yes   ``Cargo.toml`` field ``package.version``
   description   Yes   ``Cargo.toml`` field ``package.description``
   vendor
   author        Yes   ``Cargo.toml`` field ``package.authors``
   license       Yes   ``Cargo.toml`` field ``package.license``
   ============  ====  ============================================
