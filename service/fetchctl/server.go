// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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

package fetchctl

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/tomb.v2"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/service/messages"
)

type OperationRequest struct {
	Operation    string `json:"operation"`
	Type         string `json:"type,omitempty"`
	ValidateOnly bool   `json:"validate-only,omitempty"`
	Payload      string `json:"payload"`
}

type OperationReply struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func buildReply(result, message string) []byte {
	data := OperationReply{Result: result, Message: message}
	j, err := json.Marshal(data)
	if err != nil {
		return []byte(fmt.Sprintf(`{"result":"error","message":%q}`, err.Error()))
	}
	return j
}

type Server struct {
	ch     chan interface{}
	tomb   tomb.Tomb
	ctx    context.Context
	cancel context.CancelFunc
	ln     net.Listener
	mu     sync.Mutex
	conns  map[net.Conn]struct{}
}

func NewServer(ch chan interface{}) *Server {
	cs := &Server{
		ch:    ch,
		conns: make(map[net.Conn]struct{}),
	}
	cs.ctx, cs.cancel = context.WithCancel(context.Background())
	return cs
}

func (cs *Server) Start() error {
	err := os.RemoveAll(SocketPath())
	if err != nil {
		return err
	}

	logger.Info("Listening on fetchctl socket...")
	var lc net.ListenConfig
	ln, err := lc.Listen(cs.ctx, "unix", SocketPath())
	if err != nil {
		return err
	}

	cs.ln = ln

	cs.tomb.Go(func() error {
		for {
			fd, err := ln.Accept()
			if err != nil {
				// Closing the listener during shutdown makes Accept return; treat
				// that as a normal exit instead of logging a spurious error.
				select {
				case <-cs.tomb.Dying():
					return nil
				default:
				}
				logger.Errorf("cannot accept fetchctl connection: %s", err)
				continue
			}

			cs.handleConn(fd)
		}

	})

	return nil
}

func (cs *Server) handleConn(fd net.Conn) {
	cs.addConn(fd)
	defer cs.removeConn(fd)
	defer func() {
		if err := fd.Close(); err != nil {
			logger.Errorf("fetchctl: cannot close connection: %s", err)
		}
	}()

	var op OperationRequest
	var reply []byte
	dec := json.NewDecoder(fd)
	if err := dec.Decode(&op); err != nil {
		logger.Errorf("fetchctl: cannot unmarshal operation request: %s", err)
		reply = buildReply("error", err.Error())
	} else {
		logger.Infof("fetchctl: operation requested: %s", op.Operation)
		msg := messages.NewFetchCtl(op.Operation, op.Type, op.ValidateOnly, []byte(op.Payload))
		// Avoid deadlock on shutdown while sending request or waiting for reply.
		select {
		case cs.ch <- msg:
			select {
			case res := <-msg.Rch:
				reply = buildReply(res.Status, res.Message)
			case <-cs.tomb.Dying():
				reply = buildReply("error", "service shutting down")
			}
		case <-cs.tomb.Dying():
			reply = buildReply("error", "service shutting down")
		}
	}

	_, err := fd.Write(reply)
	if err != nil {
		logger.Errorf("fetchctl: cannot write reply: %s", err)
	}
}

func (cs *Server) addConn(fd net.Conn) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.conns[fd] = struct{}{}
}

func (cs *Server) removeConn(fd net.Conn) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.conns, fd)
}

func (cs *Server) closeConns() {
	cs.mu.Lock()
	conns := make([]net.Conn, 0, len(cs.conns))
	for fd := range cs.conns {
		conns = append(conns, fd)
	}
	cs.mu.Unlock()

	for _, fd := range conns {
		if err := fd.Close(); err != nil {
			logger.Errorf("fetchctl: cannot close connection: %s", err)
		}
	}
}

func (cs *Server) Stop() error {
	logger.Info("Shutting down fetchctl socket...")
	cs.tomb.Kill(nil)
	if cs.ln != nil {
		cs.ln.Close()
	}
	// Close active connections so blocked reads and writes can exit during shutdown.
	cs.closeConns()
	cs.cancel()

	removeErr := os.RemoveAll(SocketPath())

	// tomb.Wait blocks forever if no goroutine was registered via tomb.Go,
	// so only wait when Start was called (indicated by the listener being set).
	var waitErr error
	if cs.ln != nil {
		waitErr = cs.tomb.Wait()
	}
	if removeErr != nil {
		return removeErr
	}
	return waitErr
}

func (cs *Server) Dying() <-chan struct{} {
	return cs.tomb.Dying()
}

func (cs *Server) Err() error {
	return cs.tomb.Err()
}

func SocketPath() string {
	return filepath.Join(os.TempDir(), "fetchctl.socket")
}
