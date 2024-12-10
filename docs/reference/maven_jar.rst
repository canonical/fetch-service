The Maven JAR inspector
=======================

The Maven JAR inspector examines requests for JAR files hosted on a Maven
repository.


Inspector ID
------------

``maven.jar``

Internal state
--------------

None

Request verification
--------------------

The current implementation allows downloads from
``https://repo.maven.apache.org:443``. This will be changed in the future to match
an internal repository of Maven assets. Only requests with the
``/maven2/<org components separated by />/<artifact-id>/<version>/<jar file>``
form are allowed.

Acceptance criteria
-------------------

To be approved, the fetched JAR file must contain a ``pom.xml`` file with fields
that match the artifact id and version from the requested URL.

Rejection reasons
-----------------

* The requested artifact will be rejected if it's not a valid JAR file.
* The requested artifact will be rejected if the metadata inside the ``pom.xml``
  contained in the JAR file does not match the requested URL.

Extracted metadata
------------------

The following pieces of metadata are extracted by the JAR inspector:

.. table:: Maven JAR file metadata
   :widths: auto

   ============  ====  ==============================================
   Field         Used  Data source
   ============  ====  ==============================================
   name          Yes   ``pom.xml``'s ``artifact-id``
   description   Yes   ``pom.xml``'s ``description``
   vendor        Yes   ``pom.xml``'s ``group-id``
   version       Yes   ``pom.xml``'s ``version``
   author        Yes   ``pom.xml``'s ``developers``, joined with ", "
   license       Yes   ``pom.xml``'s ``licenses``, joined with " OR "
   ============  ====  ==============================================

For the ``version`` and ``vendor`` fields, the values can also come from the
``version`` and ``group-id`` XML tags present in a ``parent`` tag in the
``pom.xml`` file.
