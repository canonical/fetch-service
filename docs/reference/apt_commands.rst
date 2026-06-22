.. _ref_apt_commands:

.. meta::
    :description: Reference for the APT commands file inspector which verifies Commands files that map command names to the packages providing them.

The APT commands file inspector
===============================

APT Commands files map command names to the packages that provide them.
They are used by the ``command-not-found`` helper to suggest packages when
a user runs an unrecognised command.

This inspector currently only examines the XZ-compressed version of
the commands-<arch>.xz files.

Inspector ID
------------

``apt.commands``

Internal state
--------------

None.

Request verification
--------------------

The APT commands inspector accepts HTTP requests to the repositories
configured in the ``apt.repositories.<name>`` entry in ``inspectors.yaml``.

The commands file path is expected to match one of these patterns:

* Direct: ``/<repo>/dists/[\w-]+/[\w-]+/cnf/Commands-[\.\w-]+``
* By hash: ``/<repo>/dists/[\w-]+/[\w-]+/cnf/by-hash/SHA256/[0-9a-f]{64}``

File format
-----------

The APT commands inspector expects the Commands-<arch>.xz file to:

* Be a XZ-compressed text file.
* Contain a header block with `suite`, `component`, and `arch` entries.
* Contain blocks separated by a blank line, each block containing the
  fields ``name``, ``version``, and ``commands``.


Configuration options
---------------------

This inspector is configured under the ``apt`` key in ``inspectors.yaml``.
The ``repositories`` map defines the allowed APT repositories.

.. list-table:: Per-repository options (``apt.repositories.<name>``)
   :widths: auto
   :header-rows: 1

   * - Option
     - Description
   * - ``urls``
     - List of URL glob patterns for the repository base URL.
   * - ``suites``
     - List of allowed suite name glob patterns (for example, ``focal``
       or ``noble-updates``).
   * - ``components``
     - List of allowed component glob patterns (for example, ``main``
       or ``universe``).

Acceptance criteria
-------------------

To be approved, the artifacts examined by this inspector must comply
to the following rules:

* The ``Commands-<arch>.xz`` file must match the format described in the
  previous section.
* The ``Commands-<arch>.xz`` file must be listed in the corresponding
  ``InRelease`` file (verified by the :doc:`apt.release <apt_release>` inspector).


Rejection reasons
-----------------

The ``Commands-<arch>.xz`` file is rejected if:

* Expected fields are missing.
* The file has not been verified against the repository's ``InRelease`` file.


Extracted metadata
------------------

The following pieces of metadata are extracted by the APT commands file
inspector:

.. table:: APT commands file inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.apt.commands``
   name          Yes   ``Commands``
   version
   description   Yes   ``Commands list for command-not-found``
   vendor        Yes   ``InRelease`` field ``Origin``
   author        Yes   ``InRelease`` field ``Origin``
   author-email
   architecture
   ============  ====  ============================================

