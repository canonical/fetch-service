Service control
===============

The fetch service requires external control to operate. This document details
the API used by external controllers to command, monitor, and retrieve data
from fetch service instances.

The control API
---------------

Normal fetch service operation includes creating and ending sessions, as well
as obtaining status information and cleaning the file spool. These functionalities
are available through an API to be used by the build orchestrator.

The fetch service requires artefact requests to happen inside build sessions. Each
build session must be created by the orchestrator, and session data provided by
the fetch service must be provisioned to build instances for proper proxying setup
and authentication.

Endpoints
---------

``GET /status``
^^^^^^^^^^^^^^^

:Description:
  Obtain current service information and statistics.

:Authentication:
  Not required.

:Parameters:
  None.

:Response:

  .. code-block::

  {
      "uptime": <int>,				// service uptime in seconds
      "start-time": <string>,			// start timestamp in RFC-3339 format
      "session-count": <int>,			// total number of created sessions
      "active-sessions": [			// list of sessions currently active
          {
              "session-id": <string>,		// session ID
              "start-time": <string>,		// start timestamp in RFC-3339 format
              "policy": <string>,		// "strict" or "permissive"
              "age": <int>,			// seconds since session start
              "timeout": <int>			// session TTL in seconds	
          },
          (...)
      ],
  }


``POST /session``
^^^^^^^^^^^^^^^^^

:Description:
  Create a new fetch service session. It returns the session ID along with an authentication
  token to be used in client requests. Permissive sessions are only allowed if the fetch
  service is started in permissive mode.

:Authentication:
  Basic authentication is required to access this endpoint.

:Parameters:

  .. code-block::

    {
        "timeout": <int>,			// session timeout in seconds
        "policy": <string>		// "strict" or "permissive"
    }

:Response:

  .. code-block::

    {
        "id": <string>,			// session ID
        "token": <string>			// session token
    }


``DELETE /session/<id>/token``
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

:Description:
  Revoke the session token. A Not Found (404) error is returned if the session does
  not exist.

:Authentication:
  No authentication is required to access this endpoint, but the token to be revoked
  must be supplied as a parameter.

:Parameters:

  .. code-block::

    {
        "token": <string>
    }

:Response:

  .. code-block::

    {
        "session-id": <string>,		// session ID
        "start-time": <string>,		// session start timestamp in RFC-3339 format
        "end-time": <string>,		// session start timestamp in RFC-3339 format
        "spool-path": <string>		// file spool pathname
    }


``GET /session/<id>``
^^^^^^^^^^^^^^^^^^^^^

:Description:
  Retrieve the metadata containing a list of all downloaded artefacts. The information
  must be requested only after the session token has been revoked, and before the
  session is finished.

:Parameters:
  None.

:Authentication:
  Basic authentication is required to access this endpoint.

:Response:

  .. code-block::

    {
        "session-id": <string>,		// session ID
        "comment": <string>,            // free-form comment string
        "start-time": <string>,		// session start timestamp in RFC-3339 format
        "end-time": <string>,		// session end timestamp in RFC-3339 format
        "inspectors": <list of string>,	// list of registered inspector IDs
        "artefacts": [
          {
            "artefact-metadata-version": <string>,  // metadata compatibility (major.minor)
            "request-inspection": {
                <inspector id>: {
                    "opinion": <string>,	// "Unknown", "Rejected" or "Pending"
                    "reason": <string>,		// Explanation for opinion
                    "annotations": <inspector-specific optional map of string to any>
                },
                (...)
            }.
            "response-inspection": {
                <inspector id>: {
                    "opinion": <string>,	// "Unknown", "Rejected" or "Approved"
                    "reason": <string>,		// Explanation for opinion
                    "annotations": <inspector-specific optional map of string to any>
                },
                (...)
            },
            "result": <string>,			// "Approved" or "Rejected"
            "metadata": {
                "type": <string>,		// artefact mimetype
                "sha1": <string>,		// artefact SHA1 digest
                "sha256": <string>,		// artefact SHA256 digest
                "size": <int>,			// artefact size in bytes
                "name": <string>,		// artefact name
                "version": <string>,		// artefact version
                "vendor": <string>,		// artefact vendor
                "description": <string>,	// brief description of the artefact
                "author": <string>,		// author name
                "author-email": <string>,	// author email address
                "architecture": <string>,	// binary architecture in debian format
                "license": <string>,		// license in SPDX format
                "copyright": <string>,		// copyright information
            },
            "downloads": [
                {
                    "start-time": <string>,	// start timestamp in RFC-3339 format
                    "end-time": <string>,	// end timestamp in RFC-3339 format
                    "method": <string>,		// URL request method
                    "url": <string>,		// URL
                    "address": <string>,	// client IP address and port
                    "user-agent": <string>,	// client user agent string
                    "status-code": <int>,	// HTTP request status code
                    "status": <string>,		// textual status message
                    "content-type": <string>,	// content type informed by the server
                    "request-header": <map of string to string list>,
                    "response-header": <map of string to string list>
                },
                (...)
            ]
          },
          (...)
        ],
        "spool-path": <string>,		// file spool pathname
        "policy": <string>                // policy used in this session
    }         

``DELETE /session/<id>``
^^^^^^^^^^^^^^^^^^^^^^^^

:Description:
  Finish a session. It's not required to revoke the session token before finishing
  the session.

:Authentication:
  Basic authentication is required to access this endpoint.

:Parameters:
  None.

:Response:
  None.


``DELETE /resources/<id>``
^^^^^^^^^^^^^^^^^^^^^^^^^^

:Description:
  Remove session files from the fetch service's file spool. The session must be
  finished before resources are deleted.

:Authentication:
  Basic authentication is required to access this endpoint.

:Parameters:
  None.

:Response:
  None.

