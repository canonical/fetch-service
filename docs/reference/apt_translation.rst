.. _ref_apt_translation:

.. meta::
    :description: Reference for the APT translation file inspector which verifies translation files providing localised package descriptions.

The APT translation file inspector
==================================

APT translation files provide localised descriptions for packages in an APT
repository. They are downloaded alongside the Packages file and let
package managers display package descriptions in the user's language.

This inspector currently only examines the XZ-compressed version of
the Translation-<lang> files (they are the files downloaded when running
``apt update`` on Ubuntu).

Inspector ID
------------

``apt.translations``

Internal state
--------------

None.

Request verification
--------------------

The APT translation inspector accepts HTTP requests to the repositories
configured in the ``apt.repositories.<name>`` entry in ``inspectors.yaml``.

The translation file path is expected to match one of these patterns:

* Direct: ``/<repo>/dists/[\w-]+/[\w-]+/i18n/Translation-[\w.-]+``
* By hash: ``/<repo>/dists/[\w-]+/[\w-]+/i18n/by-hash/SHA256/[0-9a-f]{64}``

File format
-----------

The APT translation inspector expects the Translation-<lang> file to:

* Be a XZ-compressed text file.
* Contain blocks separated by a blank line, each block containing the
  fields ``Package``, ``Description-md5``, and ``Description-<lang>``.


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

* The ``Translation-<lang>.xz`` file must match the format described in the
  previous section.
* The ``Translation-<lang>.xz`` file must be listed in the corresponding
  ``InRelease`` file (verified by the :doc:`apt.release <apt_release>` inspector).


Rejection reasons
-----------------

The ``Translation-<lang>.xz`` file is rejected if:

* Expected fields are missing.
* The file has not been verified against the repository's ``InRelease`` file.


Extracted metadata
------------------

The following pieces of metadata are extracted by the APT translation file
inspector:

.. table:: APT translation file inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.apt.translation``
   name          Yes   ``Translation``
   version
   description
   vendor        Yes   ``InRelease`` field ``Origin``
   author        Yes   ``InRelease`` field ``Origin``
   author-email
   architecture
   ============  ====  ============================================

