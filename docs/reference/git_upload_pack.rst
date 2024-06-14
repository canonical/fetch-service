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

None.


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

The following pieces of metadata are extracted by the APT packages file
 inspector:

.. table:: APT release inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.git.upload-pack-result.ls-ref``,
                       ``application/x.git.upload-pack-result.fetch``
   name          Yes   ``git-upload-pack-result``
   version
   description
   vendor
   author
   author-email
   architecture
   license
   copyright
   ============  ====  ============================================

