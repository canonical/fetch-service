The Maven POM inspector
=======================

The Maven POM inspector examines requests for POM files hosted on a Maven
repository.


Inspector ID
------------

``maven.pom``

Internal state
--------------

None

Request verification
--------------------

The current implementation allows downloads from
``https://repo.maven.apache.org:443``. This will be changed in the future to match
an internal repository of Maven assets. Only requests with the
``/maven2/<org components separated by />/<artefact-id>/<version>/<pom file>``
form are allowed.

Acceptance criteria
-------------------

To be approved, the fetched POM file must have fields that match the artefact id
and version from the requested URL.

Rejection reasons
-----------------

* The requested artefact will be rejected if it's not a valid POM XML file.
* The requested artefact will be rejected if the metadata contained in the POM
  file does not match the requested URL.

Extracted metadata
------------------

The following pieces of metadata are extracted by the POM inspector:

.. table:: pom.xml metadata
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
