.. _ref_git_smart_query:

.. meta::
    :description: Reference for the git smart server query inspector which verifies the initial capability advertisement in the Git smart HTTP protocol.

Git smart query inspector
=========================

There are two HTTP-based git protocols, depending on the type of the
server being contacted. The git client verifies whether the server is 
a "smart" server (as opposed to a plain HTTP server that only supports
the "dumb" protocol) by issuing a "smart query".

The git smart query inspector verifies if the request URL is correctly
formatted, the protocol version is supported, and the response can be
decoded as a smart protocol response.


Inspector ID
------------

``git.smart-query``


Internal state
--------------

None.


Request verification
--------------------

The git smart server verification contains the ``service=git-upload-pack``
query. Valid requests contain the ``Git-Protocol`` request header set to
``version=2``.


Response format
---------------

The response is encoded using the git v2 protocol.


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

To be approved, the artifacts examined by this inspector must meet the
following criteria:

* The response sent by the server has plain text content with Content-Type
  set as ``application/x-git-upload-pack-advertisement``.
* Content is encoded using the git V2 protocol.


Rejection reasons
-----------------

The smart query request is rejected if:

* Git protocol is not V2.
* There are errors decoding the response.


Extracted metadata
------------------

The following pieces of metadata are extracted by git smart query
inspector:

.. table:: Git smart query inspector metadata
   :widths: auto

   ============  ====  ============================================
   Field         Used  Data source
   ============  ====  ============================================
   type          Yes   ``application/x.git.upload-pack-advertisement``
   name          Yes   ``git upload-pack advertisement``
   version
   description   Yes   ``response to git smart server query``
   vendor        Yes   The git server hostname
   author
   author-email
   architecture
   ============  ====  ============================================

