.. _ref_maven_pom:

.. meta::
    :description: Reference for the Maven POM inspector which verifies Maven project object model files.

The Maven POM inspector
=======================

A POM (Project Object Model) file is an XML file that describes a Maven
project, including its identity, dependencies, and build configuration.
Every artifact in a Maven repository has a corresponding POM file that
provides metadata such as the group ID, artifact ID, version, and license.

The Maven POM inspector examines requests for POM files hosted on a Maven
repository.


Inspector ID
------------

``maven.pom``

Internal state
--------------

None.

Request verification
--------------------

The Maven POM inspector recognizes requests to
``https://repo.maven.apache.org:443`` with the URL form
``/maven2/<org components separated by />/<artifact-id>/<version>/<pom file>``.
These requests are marked as unknown (unsupported origin) because
``repo.maven.apache.org`` is not a configured trusted origin.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the fetched POM file must have fields that match the artifact id
and version from the requested URL.

Rejection reasons
-----------------

* The requested artifact will be rejected if it's not a valid POM XML file.
* The requested artifact will be rejected if the metadata contained in the POM
  file does not match the requested URL.

Extracted metadata
------------------

The following pieces of metadata are extracted by the POM inspector:

.. table:: pom.xml metadata
   :widths: auto

   ============  ====  ==============================================
   Field         Used  Data source
   ============  ====  ==============================================
   type          Yes   ``text/xml``
   name          Yes   ``Maven POM file for '<artifact-id>'``
   description   Yes   ``pom.xml``'s ``description``
   vendor        Yes   ``pom.xml``'s ``group-id``
   version       Yes   ``pom.xml``'s ``version``
   author        Yes   ``pom.xml``'s ``developers``, joined with ", "
   license       Yes   ``pom.xml``'s ``licenses``, joined with " OR "
   ============  ====  ==============================================

For the ``version`` and ``vendor`` fields, the values can also come from the
``version`` and ``group-id`` XML tags present in a ``parent`` tag in the
``pom.xml`` file.
