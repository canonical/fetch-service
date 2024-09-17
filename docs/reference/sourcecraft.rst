Git sourcecraft inspector
=========================

The sourcecraft inspector examines requests against source
repositories - it validates if the cloning targed is the
proper sourcecraft repository.
This inspector only supports the smart protocol version 2.


Inspector ID
------------

``git.sourcecraft``


Internal state
--------------

None.


Request verification
--------------------

It uses the ``git.upload-pack`` result to check the entry requirements:

* if clone is shallow
* if single revision is requested
* if data is a reponse to the ``fetch`` command

After the initial validation it tries to extract the packed content of the repository.
In the extracted repository it looks for the ``sourcecraft.yaml`` file and loads it
for further inspection. File is used to extract the metadata of the processed artefact.


Acceptance criteria
-------------------

* Content format is binary.
* Content-type is declared as ``application/x-git-upload-pack-result``.
* Content can be decoded using the git protocol v2.
* It must be a shallow fetch.
* It must request a single ref.
* Repository contains the ``sourcecraft.yaml`` file.
* ``sourcecraft.yaml`` can be read and contains valid basic fields.


Rejection reasons
-----------------

* repository clone is not shallow
* ``sourcecraft.yaml`` cannot be read
* ``sourcecraft.yaml`` does not have required metadata


Extracted metadata
------------------

The following pieces of metadata are extracted by the Sourcecraft inspector:

.. table:: Sourcecraft inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.git.sourcecraft``
   name          Yes   ``sourcecraft.yaml`` field ``name``
   version       Yes   ``sourcecraft.yaml`` field ``version``
   description   Yes   ``sourcecraft.yaml`` field ``summary``
   vendor
   author
   author-email
   architecture
   license       Yes   ``sourcecraft.yaml`` field ``license``
   copyright
   ============  ====  ============================================

