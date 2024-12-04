Writing a fetch service inspector
=================================

The inspector interface defines methods to examine artifact requests and
downloaded data before they are sent to requesting client. Artifact requests
can be rejected or held for further investigation, and only after a full file
inspection an artifact can be set as approved.


How to inspect a request
------------------------

At request inspecting time the inspector emits an opinion on whether the
request should be allowed to reach the HTTP server, or be blocked.

The inspector's ``InspectRequest`` method is used to verify whether the
HTTP request is valid. If no opinion is emitted the fetch service assumes that
the request was ignored by this inspector. The ``SetRequestPending`` artifact
method is used to set the request inspection opinion to ``Pending`` when the
request seems correct and the artifact must be inspected.

An inspector can block a request by using ``SetRequestRejected`` to set the
opinion to ``Rejected``, but this must be done carefully. If an inspector
rejects a request, the artifact won't be downloaded even if another inspector
sets it to pending state.

Here is an example on how to implement a request inspector that only allows
requests to a given host::

  import (
      . "github.com/canonical/fetch-service/inspectors/common"
  )

  func (ins *ExampleInspector) InspectRequest(a RequestArtifact) error {
          u, err := url.Parse(a.CurrentDownload.URL)
          if err != nil {
	          return fmt.Errorf("cannot parse URL: %s", err)
 	  }
          if u.Host == validHost {
                  a.SetRequestPending(ins, "URL has a valid origin")
          }
          return nil
  }


How to inspect an artifact
--------------------------

After the artifact is downloaded, it can be examined for metadata extraction.
Although it was cleared by the request inspector, the inspector must make sure
the file is what it expects it to be before approving _or_ rejecting it. Inspectors
must emit only one opinion about a recognized artifact. The ``SetResponseApproved``
and ``SetResponseRejected`` artifact methods must be used to set the inspector's
opinion to ``Approved`` or ``Rejected``.

This example shows how to implement an artifact inspector that examines a JSON file,
extracts author metadata, and only allows documents created by a specific author to reach
the requesting client::

  import (
      . "github.com/canonical/fetch-service/inspectors/common"
  )

  func (ins *JsonInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
          if !a.MimetypeIs("application/json") {
                  return nil  // we don't recognize this artifact
          }

          data, err := os.ReadAll(f)
          if err != nil {
                  return err  // something went wrong during the inspection
          }

          if err := json.Unmarshal(data, &payload); err != nil {
                  return err
          }

          a.SetArtifactMetadata(ArtifactMetadata{
                Type:   "application/json",
                Author: payload.Author,
          }

          if payload.Author == "Foo J. Bar" {
                  a.SetResponseApproved(ins, "this is a known author")
          } else {          
                  a.SetResponseRejected(ins, "unrecognized author name")
          }

          return nil
  }


How to add annotations to an opinion
------------------------------------

Opinions can be annotated with inspector-specific information. Annotations
can be used to add context to an opinion and are intended for human consumption::

  if payload.Author == "Foo J. Bar" {
          a.SetResponseApproved(ins, "this is a known author").Annotate(
                  Annotation{
                          "keywords": payload.Keywords,
                          "creation-date": payload.CreationDate,
                  }
          )
  }

Annotations can also be added individually::

  notes := Annotation{}
  notes.Add("expiration-date", payload.ExpirationDate)


How to perform stateful artifact inspection
-------------------------------------------

Stateful artifact inspection is used when an artifact needs to be validated
against another artifact. One example of such case would be an artifact that
is valid only if its digest is listed in a previously examined index artifact.
To do that, the inspector keeps information from the index artifact inspection
and checks other artifacts against this internal state::

  type FileInspector struct {
          validDigests []string
          lock sync.Mutex
  }

  func (ins *FileInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
          if !knownFileFormat(a) {
                  return nil  // we don't recognize this file format
          }

          if data, err := checkIfIndexFile(a); err == nil {
                  ins.lock.Lock()
                  defer ins.lock.Unlock()
                  ins.validDigests = data.ValidDigests
                  return nil
          }

          if slices.Contains(ins.validDigests, a.Sha256().String()) {
                  a.SetResponseApproved(ins, "file digest is valid")
          } else {
                  a.SetResponseRejected(ins, "file digest not listed in the index file")
          }

          return nil
  }

Note that inspectors may run concurrently and operations on inspector states
must be done in a thread-safe way to avoid races.

