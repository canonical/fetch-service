Writing a fetch service inspector
=================================

The inspector interface defines methods to examine artefact requests and
downloaded data before they are sent to requesting client. Artefact requests
can be rejected or held for further investigation, and only after a full file
inspection an artefact can be set as approved.


How to inspect a request
------------------------

At request inspecting time the inspector emits an opinion on whether the
request should be allowed to reach the HTTP server, or be blocked.

The inspector's ``InspectRequest`` method is used to verify whether the
HTTP request is valid. If no opinion is emitted the fetch service assumes that
the request was ignored by this inspector. The ``Hold`` artefact method is
used when the request seems correct and the artefact must be inspected.

An inspector can block a request using the ``Reject`` artefact method, but
this must be done carefully. If an inspector rejects a request, the artefact
won't be downloaded even if another inspector sets it to pending state.

Here is an example on how to implement a request inspector that only allows
requests to a given host::

  func (ins *ExampleInspector) InspectRequest(a *metadata.Artefact) error {
          u, err := url.Parse(a.CurrentDownload.URL)
          if err != nil {
	          return fmt.Errorf("cannot parse URL: %s", err)
 	  }
          if u.Host == validHost {
                  a.Hold(ins, "URL has a valid origin")
          }
          return nil
  }


How to inspect an artefact
--------------------------

After the artefact is downloaded, it can be examined for metadata extraction.
Although it was cleared by the request inspector, the inspector must make sure
the file is what it expects it to be before approving _or_ rejecting it. Inspectors
must emit only one opinion about a recognized artefact.

This example shows how to implement an artefact inspector that examines a JSON file,
extracts author metadata, and only allows documents created by a specific author to reach
the requesting client::

  func (ins *JsonInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
          if !a.MimeType.Is("application/json") {
                  return nil  // we don't recognize this artefact
          }

          data, err := os.ReadAll(f)
          if err != nil {
                  return err  // something went wrong during the inspection
          }

          if err := json.Unmarshal(data, &payload); err != nil {
                  return err
          }

          a.Metadata.Author = payload.Author

          if payload.Author == "Foo J. Bar" {
                  a.Approve(ins, "this is a known author") 
          } else {          
                  a.Reject(ins, "unrecognized author name") 
          }

          return nil
  }


How to add annotations to an opinion
------------------------------------

Opinions can be annotated with inspector-specific information. Annotations
can be used to add context to an opinion and are intended for human consumption::

  if payload.Author == "Foo J. Bar" {
          a.Approve(ins, "this is a good author").Annotate(
                  metadata.Annotation{
                          "keywords": payload.Keywords,
                          "creation-date": payload.CreationDate,
                  }
          )
  }

Annotations can also be added individually::

  notes := metadata.Annotation{}
  notes.Add("expiration-date", payload.ExpirationDate)


How to perform stateful artefact inspection
-------------------------------------------

Stateful artefact inspection is used when an artefact needs to be validated
against another artefact. One example of such case would be an artefact that
is valid only if its digest is listed in a previously examined index artefact.
To do that, the inspector keeps information from the index artefact inspection
and checks other artefacts against this internal state::

  type FileInspector struct {
          validDigests []string
  }

  func (ins *FileInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
          if !knownFileFormat(a) {
                  return nil  // we don't recognize this file format
          }

          if data, err := CheckIfIndexFile(a); err == nil {
                  ins.validDigests = data.ValidDigests
                  return nil
          }

          if slices.Contains(ins.validDigests, a.Metadata.Sha256.String()) {
                  a.Approve(ins, "file digest is valid")
          } else {
                  a.Reject(ins, "file digest not listed in the index file")
          }

          return nil
  }

Note that inspectors may run concurrently and operations on inspector states
must be done in a thread-safe way to avoid races.

