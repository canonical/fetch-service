.. _ref_lxd_instance_types:

.. meta::
    :description: Reference for the LXD instance types inspector which verifies the instance types YAML file from the LXD metadata server.

The LXD instance types inspector
=================================

LXD instance types are named resource profiles that map common cloud
provider instance sizes to CPU and memory allocations. LXD uses them to
create instances with standardised configurations using familiar
cloud provider terminology.

The LXD instance types inspector examines instance type YAML files served
by the LXD image metadata server.

Inspector ID
------------

``lxd.instance-types``

Internal state
--------------

None.

Request verification
--------------------

The LXD instance types inspector accepts HTTPS requests to
``https://images.lxd.canonical.com:443`` with a path matching
``/meta/instance-types/*.yaml``.

File format
-----------

The inspector recognizes three YAML file formats:

* **Consolidated**: a map of provider names, each containing a map of
  instance names to objects with non-empty ``cpu`` fields.
* **Single-provider**: a map of instance names to objects with non-empty
  ``cpu`` fields.
* **Index**: a map of provider names to YAML filenames (values must end
  in ``.yaml``).

Configuration options
---------------------

None.

Acceptance criteria
-------------------

To be approved, the artifact must:

* Have content type ``text/plain``.
* Match one of the recognized YAML formats with at least one entry.

Rejection reasons
-----------------

The artifact is rejected or ignored if:

* The content type is not ``text/plain``.
* The file does not match any recognized YAML format.
* No entries are found in the parsed data.

Extracted metadata
------------------

The following pieces of metadata are extracted by the LXD instance types
inspector:

.. table:: LXD instance types inspector metadata
   :widths: auto

   ============  ====  =============================================
   Field         Used  Data source
   ============  ====  =============================================
   type          Yes   ``application/x.canonical.lxd-instance-types``
   name          Yes   ``Instance types``
   description   Yes   ``LXD instance types metadata``
   version
   vendor
   author
   ============  ====  =============================================
