.. _ref_maven_jar:

.. meta::
    :description: Reference for the Maven JAR inspector which verifies Java archive artifacts from a Maven repository.

The Maven JAR inspector
=======================

A JAR (Java Archive) file is a zip-format archive containing compiled Java
classes and associated resources. JAR files are the standard distribution
format for Java libraries and are hosted on Maven-compatible repositories
using a well-defined path structure based on group ID, artifact ID, and
version.

The Maven JAR inspector examines requests for JAR files hosted on a Maven
repository.


Inspector ID
------------

``maven.jar``

Internal state
--------------

None.

Request verification
--------------------

The Maven JAR inspector recognizes requests to
``https://repo.maven.apache.org:443`` with the URL form
``/maven2/<org components separated by />/<artifact-id>/<version>/<jar file>``.
These requests are marked as unknown (unsupported origin) because
``repo.maven.apache.org`` is not a configured trusted origin.

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the fetched JAR file must contain a ``pom.xml`` file at
``META-INF/maven/<group-id>/<artifact-id>/pom.xml``. The ``<artifactId>``
element in that file must match the artifact ID derived from its path within
the JAR. The version is not verified against the requested URL.

Rejection reasons
-----------------

* The requested artifact will be rejected if it's not a valid JAR file.
* The requested artifact will be rejected if the ``<artifactId>`` element in
  the embedded ``pom.xml`` does not match the artifact ID in the file's path
  within the JAR archive.

Extracted metadata
------------------

The following pieces of metadata are extracted by the JAR inspector:

.. table:: Maven JAR file metadata
   :widths: auto

   ============  ====  ==============================================
   Field         Used  Data source
   ============  ====  ==============================================
   type          Yes   ``application/jar``
   name          Yes   ``pom.xml``'s ``artifactId``
   description   Yes   ``pom.xml``'s ``description``
   vendor        Yes   ``pom.xml``'s ``group-id``
   version       Yes   ``pom.xml``'s ``version``
   author        Yes   ``pom.xml``'s ``developers``, joined with ", "
   license       Yes   ``pom.xml``'s ``licenses``, joined with " OR "
   ============  ====  ==============================================

For the ``version`` and ``vendor`` fields, the values can also come from the
``version`` and ``group-id`` XML tags present in a ``parent`` tag in the
``pom.xml`` file.
