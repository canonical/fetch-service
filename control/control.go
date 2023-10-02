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

type SessionData struct {
	ID    string
	Token string
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
	router.HandleFunc("/new-session", c.createSession)
	router.HandleFunc("/end-session/{id}", c.endSession)

	logger.Infof("control server listening on %s\n", addr)

	go func() {
		logger.Fatal(http.ListenAndServe(addr, router))
	}()
}

func (c *Server) createSession(w http.ResponseWriter, r *http.Request) {
	msg := messages.NewCreateSession()
	c.ch <- msg
	cred := <-msg.Rch
	j, err := json.Marshal(cred)
	if err != nil {
		panic(err) // XXX
	}
	w.Write(j)
}

func (c *Server) endSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		panic("not ok")
	}
	msg := messages.NewEndSession(id)
	c.ch <- msg
	result := <-msg.Rch
	j, err := json.Marshal(result)
	if err != nil {
		panic(err) // XXX
	}
	w.Write(j)
}
