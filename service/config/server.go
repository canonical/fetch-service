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

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"gopkg.in/tomb.v2"

	"github.com/canonical/fetch-service/logger"
)

type OperationRequest struct {
	Operation    string `json:"operation"`
	Type         string `json:"type"`
	ValidateOnly bool   `json:"validate-only"`
	Payload      string `json:"payload"`
}

type OperationReply struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func buildReply(result, message string) []byte {
	data := OperationReply{result, message}
	j, err := json.Marshal(data)
	if err != nil {
		return []byte(fmt.Sprintf(`{"result":"error","message":%q}`, err.Error()))
	}
	return j
}

type Server struct {
	tomb   tomb.Tomb
	ctx    context.Context
	cancel context.CancelFunc
}

func NewServer() *Server {
	cs := &Server{}
	cs.ctx, cs.cancel = context.WithCancel(context.Background())
	return cs
}

func (cs *Server) Start() error {
	err := os.RemoveAll(socketPath())
	if err != nil {
		return err
	}

	logger.Info("Listening on configuration socket...")
	var lc net.ListenConfig
	ln, err := lc.Listen(cs.ctx, "unix", socketPath())
	if err != nil {
		return err
	}

	cs.tomb.Go(func() error {
		for {

			fd, err := ln.Accept()
			if err != nil {
				logger.Errorf("cannot accept configuration connection: %s", err)
				continue
			}

			var op OperationRequest
			var reply []byte
			dec := json.NewDecoder(fd)
			if err := dec.Decode(&op); err != nil {
				logger.Errorf("[config] cannot unmarshal operation request: %s", err)
				reply = buildReply("error", err.Error())
			} else {
				logger.Infof("[config] operation requested: %s", op.Operation)
				reply = buildReply("ok", "operation successful")
			}

			_, err = fd.Write(reply)
			if err != nil {
				logger.Errorf("[config] cannot write configuration reply: %s", err)

			}
		}

	})

	return nil
}

func (cs *Server) Stop() error {
	logger.Info("Shutting down configuration socket...")
	cs.cancel()
	<-cs.ctx.Done()

	err := os.RemoveAll(socketPath())
	if err != nil {
		return err
	}
	cs.tomb.Kill(nil)

	return nil
}

func (cs *Server) Dying() <-chan struct{} {
	return cs.tomb.Dying()
}

func socketPath() string {
	return filepath.Join(os.TempDir(), "fetchctl.socket")
}
