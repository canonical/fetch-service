// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package control

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/utils"
)

// Parameters for session creation
type createSessionParameters struct {
	Timeout uint64 `json:"timeout"` // Session timeout in seconds
	Policy  string `json:"policy"`  // Session policy ("strict" or "permissive")
}

// Parameters for token revocation
type revokeTokenParameters struct {
	Token string `json:"token"` // The token to revoke
}

type Server struct {
	port int
	ch   chan interface{}
	user string
	pw   string
}

func NewServer(port int, ch chan interface{}, creds string) *Server {
	v := strings.SplitN(creds, ":", 2)
	if len(v) != 2 {
		v = []string{"", ""}
	}
	return &Server{
		port: port,
		ch:   ch,
		user: v[0],
		pw:   v[1],
	}
}

func (c *Server) Start() {
	addr := fmt.Sprintf(":%d", c.port)
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/status", c.getServiceStatus).Methods("GET")
	router.HandleFunc("/session", c.createSession).Methods("POST")
	router.HandleFunc("/session/{id}/token", c.deleteSessionToken).Methods("DELETE")
	router.HandleFunc("/session/{id}", c.getSessionReport).Methods("GET")
	router.HandleFunc("/session/{id}", c.deleteSession).Methods("DELETE")
	router.HandleFunc("/resources/{id}", c.deleteResources).Methods("DELETE")

	logger.Infof("control server listening on %s\n", addr)

	go func() {
		logger.Fatal(http.ListenAndServe(addr, router))
	}()
}

func (c *Server) getServiceStatus(w http.ResponseWriter, r *http.Request) {
	logger.Debugf("get service status")

	msg := messages.NewGetServiceStatus()
	c.ch <- msg
	status := <-msg.Rch
	j, err := json.Marshal(status)
	if err != nil {
		internalServerError(w, r)
		return
	}

	write_response(w, j)
}

func (c *Server) createSession(w http.ResponseWriter, r *http.Request) {
	if !c.checkAuth(w, r) {
		return
	}

	var params createSessionParameters
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		badRequest(w, r, err.Error())
		return
	}

	logger.Debugf("create session parameters: %+v\n", params)

	msg := messages.NewCreateSession(params.Policy, params.Timeout)
	c.ch <- msg
	cred := <-msg.Rch
	if cred.Err != nil {
		badRequest(w, r, cred.Err.Error())
		return
	}

	j, err := json.Marshal(cred)
	if err != nil {
		internalServerError(w, r)
		return
	}

	write_response(w, j)
}

func (c *Server) deleteSessionToken(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		notFound(w, r)
		return
	}

	var params revokeTokenParameters
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		badRequest(w, r, err.Error())
		return
	}

	logger.Debug("revoke token")
	msg := messages.NewRevokeToken(id, params.Token)
	c.ch <- msg
	res := <-msg.Rch

	if res.Err != nil {
		if res.Err == messages.ErrSessionNotFound {
			notFound(w, r)
		} else if res.Err == messages.ErrInvalidSessionToken {
			badRequest(w, r, res.Err.Error())
		} else {
			internalServerError(w, r)
		}
		return
	}

	j, err := json.Marshal(res)
	if err != nil {
		internalServerError(w, r)
		return
	}

	write_response(w, j)
}

func (c *Server) getSessionReport(w http.ResponseWriter, r *http.Request) {
	if !c.checkAuth(w, r) {
		return
	}

	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		notFound(w, r)
		return
	}

	logger.Debugf("get session %s data\n", id)

	msg := messages.NewSessionReport(id)
	c.ch <- msg
	res := <-msg.Rch

	if res.Err != nil {
		if res.Err == messages.ErrSessionNotFound {
			notFound(w, r)
		} else if res.Err == messages.ErrSessionActive {
			badRequest(w, r, res.Err.Error())
		} else {
			internalServerError(w, r)
		}
		return
	}

	j, err := utils.JSONMarshalNoHTMLEscape(res)
	if err != nil {
		internalServerError(w, r)
		return
	}

	write_response(w, j)
}

func (c *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	if !c.checkAuth(w, r) {
		return
	}

	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		notFound(w, r)
		return
	}

	logger.Debugf("end session %s\n", id)

	msg := messages.NewEndSession(id)
	c.ch <- msg
	err := <-msg.Rch

	if err != nil {
		if err == messages.ErrSessionNotFound {
			notFound(w, r)
		} else {
			internalServerError(w, r)
		}
		return
	}
}

func (c *Server) deleteResources(w http.ResponseWriter, r *http.Request) {
	if !c.checkAuth(w, r) {
		return
	}

	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		notFound(w, r)
		return
	}

	logger.Debugf("delete resources from session %s\n", id)

	msg := messages.NewDeleteResources(id)
	c.ch <- msg
	err := <-msg.Rch

	if err != nil {
		if err == messages.ErrSessionNotFound {
			notFound(w, r)
		} else if err == messages.ErrSessionActive {
			badRequest(w, r, err.Error())
		} else {
			internalServerError(w, r)
		}
	}
}

// checkAuth verifies basic authentication for requests.
func (c *Server) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	user, pw, ok := r.BasicAuth()
	if !ok {
		logger.Debugf("basic auth decoding failed")
		unauthorized(w, r)
		return false
	}

	if user != c.user || pw != c.pw {
		unauthorized(w, r)
		return false
	}

	return true
}

func badRequest(w http.ResponseWriter, r *http.Request, reason string) {
	logger.Warningf("400 Bad Request HTTP error: %s", r.URL)
	w.WriteHeader(http.StatusBadRequest)
	write_response(w, []byte(reason))
}

func internalServerError(w http.ResponseWriter, r *http.Request) {
	logger.Warningf("500 Internal Server Error HTTP error: %s", r.URL)
	w.WriteHeader(http.StatusInternalServerError)
	write_response(w, []byte("500 Internal Server Error"))
}

func notFound(w http.ResponseWriter, r *http.Request) {
	logger.Warningf("404 Not Found HTTP error: %s", r.URL)
	w.WriteHeader(http.StatusNotFound)
	write_response(w, []byte("404 Not Found"))
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	logger.Warningf("401 unauthorized HTTP error: %s", r.URL)
	w.WriteHeader(http.StatusUnauthorized)
	write_response(w, []byte("401 Unauthorized"))
}

func write_response(w http.ResponseWriter, b []byte) {
	logger.Debugf("control API response: %s\n", b)
	var err error
	_, err = w.Write(b)
	if err != nil {
		logger.Errorf("write response error: %s", err)
	}
}
