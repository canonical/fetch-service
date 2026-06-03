.. _ref_git_upload_pack:

.. meta::
    :description: Reference for the git upload-pack inspector which verifies repository clones using the Git upload-pack protocol.

Git upload pack inspector
=========================

Git issues commands to list and retrieve repository content using the
upload-pack request. This inspector only supports the smart protocol
version 2.


Inspector ID
------------

``git.upload-pack``


Internal state
--------------

* A map of repository URLs to their list of heads (branch names to commit
  hashes), populated from ``ls-refs`` responses.
* A map of repository URLs to their list of tags (tag names to commit
  hashes), populated from ``ls-refs`` responses.


Request verification
--------------------

The HTTP request to the upload-pack declares the git protocol version in
the ``Git-Protocol`` header entry. The content type for upload-pack
requests is ``application/x-git-upload-pack-request``, and the client
must accept ``application/x-git-upload-pack-result``.

The request body is encoded according to the git protocol and states the
command to be executed. Only the ``ls-refs`` and ``fetch`` commands are
recognized. To correctly log the content being transferred, only shallow,
single-ref requests are approved by the inspector.


Response format
---------------

The git upload-pack response is encoded according to the git protocol v2.


Configuration options
---------------------

This inspector is configured under the ``git`` key in ``inspectors.yaml``.

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

If it's a response to ``ls-refs``, the following format is expected:

* Content format is plain text.
* Content-type is declared as ``application/x-git-upload-pack-result``.
* Content can be decoded using the git protocol v2.

In the case of the ``fetch`` command, the response has the following
format:

* Content format is binary.
* Content-type is declared as ``application/x-git-upload-pack-result``.
* Content can be decoded using the git protocol v2.
* It must be a shallow fetch.
* It must request a single ref.



Rejection reasons
-----------------

The upload-pack response is rejected if:

* Content-type is not ``application/x-git-upload-pack-result``.
* There are errors decoding the protocol.
* The fetch depth is greater than 1.
* There are multiple refs being fetched.


Extracted metadata
------------------

The following pieces of metadata are extracted by the git upload-pack
inspector:

.. table:: Git upload-pack inspector metadata
   :widths: auto

   ============  ====  ==================================================
   Field         Used  Data source
   ============  ====  ==================================================
   type          Yes   ``application/x.git.upload-pack-result.ls-ref``
                       (ls-refs response),
                       ``application/x.git.upload-pack-result.fetch``
                       (fetch response)
   name          Yes   ``git ls-refs response`` (ls-refs),
                       ``git fetch response`` (fetch)
   version
   description   Yes   ``Response to the git '<cmd>' command``
   vendor        Yes   The git server hostname
   author
   author-email
   architecture
   ============  ====  ==================================================

