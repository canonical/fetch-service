Sourcecraft inspector
=========================

The Sourcecraft inspector checks whether the git
repository that is selected for cloning is a valid sourcecraft repository.

Once a request is accepted for inspection it tries to extract the packed content
of the repository. In the extracted data it looks for the ``sourcecraft.yaml``
file, which if found is used to extract the metadata of the processed project.

This inspector has to be called after the ``git.upload-pack`` inspector.
This inspector only supports the git v2 smart protocol.

Inspector ID
------------

``craft.sourcecraft``


Internal state
--------------

None.


Request verification
--------------------

A request is approved for the further inspection if it meets all the
following criteria:

* Request comes from a known origin

* `Content-Type <https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type>`_
  HTTP header field is declared as ``application/x-git-upload-pack-request``

* `Accept <https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept>`_
  HTTP header field is declared as ``application/x-git-upload-pack-result``

* Request is a response to the ``fetch`` command.



Acceptance criteria
-------------------

A repository is approved if the processed artefact meets all of the following criteria:

* `Content-Type <https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type>`_
  of the response is declared as ``application/x-git-upload-pack-result``

* The clone is
  `shallow <https://git-scm.com/docs/git-clone#Documentation/git-clone.txt-code--depthcodeemltdepthgtem>`_.

* The request is for a single revision.
* The data is a response to the ``fetch`` command.
* Content can be decoded using the git protocol v2.
* Repository contains the ``sourcecraft.yaml`` file.
* ``sourcecraft.yaml`` can be read and contains valid basic fields.


Rejection reasons
-----------------

* The clone is not
  `shallow <https://git-scm.com/docs/git-clone#Documentation/git-clone.txt-code--depthcodeemltdepthgtem>`_

* Multiple git revisions were requested
* ``sourcecraft.yaml`` cannot be read
* ``sourcecraft.yaml`` does not have required metadata fields


Extracted metadata
------------------

The following pieces of metadata are extracted by the Sourcecraft inspector:

.. table:: Sourcecraft inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.craft.sourcecraft``
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

