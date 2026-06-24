.. _ref_bincraft:

.. meta::
    :description: Reference for the Bincraft inspector which verifies repositories containing bincraft projects cloned using the git upload-pack protocol.

Bincraft inspector
==================

The Bincraft inspector checks whether the Git repository that was selected for cloning
is a valid Bincraft repository.

Once a request is accepted for inspection it downloads and tries to extract the packed content
of the repository. In the extracted data it looks for the ``bincraft.yaml``
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

``craft.bincraft``


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

* The upload-pack command is ``fetch``.



Configuration options
---------------------

This inspector is configured under the ``crafts`` key in ``inspectors.yaml``.

.. list-table::
   :widths: auto
   :header-rows: 1

   * - Option
     - Description
   * - ``urls``
     - List of URL glob patterns. Only requests to matching URLs are
       approved for further inspection.

Acceptance criteria
-------------------

A repository is approved if the processed artifact meets all of the following criteria:

* The response's `Content-Type <https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type>`_
  HTTP header is ``application/x-git-upload-pack-result``.
* The clone is
  `shallow <https://git-scm.com/docs/git-clone#Documentation/git-clone.txt-code--depthcodeemltdepthgtem>`_.
* The request is for a single revision.
* The data is a response to the ``fetch`` upload-pack command.
* The content can be decoded with v2 of Git's smart transfer protocol.
* The repository contains a ``bincraft.yaml`` file.
* The ``bincraft.yaml`` file is readable and can be decoded as YAML.


Rejection reasons
-----------------

A repository is rejected if any of the following criteria are met:

* The clone is not
  `shallow <https://git-scm.com/docs/git-clone#Documentation/git-clone.txt-code--depthcodeemltdepthgtem>`_.
* Multiple Git revisions were requested.
* The ``bincraft.yaml`` file is unreadable.

If the repository is missing the ``bincraft.yaml`` file,
the inspector returns no verdict and defers to subsequent inspectors.


Extracted metadata
------------------

The following pieces of metadata are extracted by the Bincraft inspector:

.. table:: Bincraft inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.canonical.bincraft``
   name          Yes   ``name`` key in ``bincraft.yaml``
   version       Yes   ``version`` key in ``bincraft.yaml``
   description   Yes   ``summary`` key in ``bincraft.yaml``
   vendor
   author
   author-email
   architecture
   license       Yes   ``license`` key in ``bincraft.yaml``
   content-id    Yes   Fetched Git ref
   ============  ====  ============================================
