// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023 Canonical Ltd.
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

	"github.com/gorilla/mux"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/service/messages"
)

type createSessionParameters struct {
	Timeout uint64 `json:"timeout"`
	Policy  string `json:"policy"`
}

type endSessionParameters struct {
}

type deleteResourcesParameters struct {
}

type Server struct {
	port int
	ch   chan interface{}
}

func NewServer(port int, ch chan interface{}) *Server {
	return &Server{port: port, ch: ch}
}

func (c *Server) Start() {
	addr := fmt.Sprintf(":%d", c.port)
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/status", c.getServiceStatus).Methods("GET")
	router.HandleFunc("/session", c.createSession).Methods("POST")
	router.HandleFunc("/session/{id}", c.endSession).Methods("DELETE")
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
	w.Write(j)
}

func (c *Server) createSession(w http.ResponseWriter, r *http.Request) {
	logger.Debugf("create session")

	var params createSessionParameters
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		internalServerError(w, r)
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
	w.Write(j)
}

func (c *Server) endSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		notFound(w, r)
		return
	}

	var params endSessionParameters
	// if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
	//	internalServerError(w, r)
	//	return
	// }

	logger.Debugf("end session %s: %+v\n", id, params)

	msg := messages.NewEndSession(id)
	c.ch <- msg
	result := <-msg.Rch
	j, err := json.Marshal(result)
	if err != nil {
		internalServerError(w, r)
		return
	}

	w.Write(j)
}

func (c *Server) deleteResources(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		notFound(w, r)
		return
	}

	var params deleteResourcesParameters
	// if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
	//	internalServerError(w, r)
	//	return
	// }

	logger.Debugf("delete resources from session %s: %+v\n", id, params)

	msg := messages.NewDeleteResources(id)
	c.ch <- msg
	err := <-msg.Rch
	if err != nil {
		if err == messages.ErrSessionActive {
			badRequest(w, r, err.Error())
		} else {
			internalServerError(w, r)
		}
	}
}

func badRequest(w http.ResponseWriter, r *http.Request, reason string) {
	logger.Warningf("bad request response: %s", r.URL)
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(reason))
}

func internalServerError(w http.ResponseWriter, r *http.Request) {
	logger.Warningf("internal server error response: %s", r.URL)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("500 Internal Server Error"))
}

func notFound(w http.ResponseWriter, r *http.Request) {
	logger.Warningf("not found response: %s", r.URL)
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("404 Not Found"))
}
