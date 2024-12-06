// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024 Canonical Ltd.
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
}

func NewServer(ch chan interface{}) *Server {
	cs := &Server{ch: ch}
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

	cs.tomb.Go(func() error {
		for {
			fd, err := ln.Accept()
			if err != nil {
				logger.Errorf("cannot accept fetchctl connection: %s", err)
				continue
			}

			var op OperationRequest
			var reply []byte
			dec := json.NewDecoder(fd)
			if err := dec.Decode(&op); err != nil {
				logger.Errorf("[fetchctl] cannot unmarshal operation request: %s", err)
				reply = buildReply("error", err.Error())
			} else {
				logger.Infof("[fetchctl] operation requested: %s", op.Operation)
				msg := messages.NewFetchCtl(op.Operation, op.Type, op.ValidateOnly, []byte(op.Payload))
				cs.ch <- msg
				res := <-msg.Rch
				reply = buildReply(res.Status, res.Message)
			}

			_, err = fd.Write(reply)
			if err != nil {
				logger.Errorf("[fetchctl] cannot write fetchtl reply: %s", err)
			}
			if err := fd.Close(); err != nil {
				logger.Errorf("[fetchctl] cannot close connection: %s", err)
			}
		}

	})

	return nil
}

func (cs *Server) Stop() error {
	logger.Info("Shutting down fetchctl socket...")
	cs.cancel()
	<-cs.ctx.Done()

	err := os.RemoveAll(SocketPath())
	if err != nil {
		return err
	}
	cs.tomb.Kill(nil)

	return nil
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
