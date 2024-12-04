Rockcraft inspector
===================

The Rockcraft inspector checks whether the Git
repository that was selected for cloning is a valid and trustworthy
Rockcraft repository.

Once a request is accepted for inspection it downloads and tries to extract the packed content
of the repository. In the extracted data it looks for the ``rockcraft.yaml``
file, which if found is used to extract the metadata of the processed project.

If the metadata is missing or doesn't properly attest for the code in the
repository, the proxy rejects the request, protecting the calling machine from
accessing the code.

This inspector must be called after the :doc:`git.upload-pack
<git_upload_pack>` inspector, and only supports v2 of the `smart transfer
protocol
<https://git-scm.com/book/en/v2/Git-Internals-Transfer-Protocols#_the_smart_protocol>`_
in Git.

Inspector ID
------------

``craft.rockcraft``


Internal state
--------------

None.


Request verification
--------------------

A request is approved for further inspection if it meets all the
following criteria:

* The request comes from a known origin.

* The `Content-Type <https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type>`_
  HTTP header is ``application/x-git-upload-pack-request``.

* The `Accept <https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept>`_
  HTTP header is ``application/x-git-upload-pack-result``.

* The request is a response to the ``fetch`` command.



Acceptance criteria
-------------------

A repository is approved if the processed artifact meets all of the following criteria:

* The response's `Content-Type <https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type>`_
  HTTP header is ``application/x-git-upload-pack-result``.
* The clone is
  `shallow <https://git-scm.com/docs/git-clone#Documentation/git-clone.txt-code--depthcodeemltdepthgtem>`_.
* The request is for a single revision.
* The data is a response to the ``fetch`` command.
* The content can be decoded with v2 of Git's smart transfer protocol.
* The repository contains a ``rockcraft.yaml`` file.
* The ``rockcraft.yaml`` file is readable and contains valid metadata keys.


Rejection reasons
-----------------

A repository is rejected if any of the following criteria are met:

* The clone is not
  `shallow <https://git-scm.com/docs/git-clone#Documentation/git-clone.txt-code--depthcodeemltdepthgtem>`_.
* Multiple Git revisions were requested.
* The ``rockcraft.yaml`` file is unreadable.
* The ``rockcraft.yaml`` file doesn't include the required metadata keys.

If the repository is missing the ``rockcraft.yaml`` file, the inspector can't
determine validity on its own, and redirects the request for the further
inspection.


Extracted metadata
------------------

The following pieces of metadata are extracted by the Rockcraft inspector:

.. table:: Rockcraft inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.canonical.rockcraft``
   name          Yes   ``name`` key in ``rockcraft.yaml``
   version       Yes   ``version`` key in ``rockcraft.yaml`` (does not currently support ``adopt-info``)
   description   Yes   ``summary`` key in ``rockcraft.yaml``
   vendor
   author
   author-email
   architecture
   license       Yes   ``license`` key in ``rockcraft.yaml``
   copyright
   ============  ====  ============================================

