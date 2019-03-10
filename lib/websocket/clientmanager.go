// Copyright 2018, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"context"
	"sync"

	"github.com/shuLhan/share/lib/ints"
)

//
// ClientManager manage list of active websocket connections on server.
//
// This library assume that each connection belong to a user in the server,
// where each user is representated by uint64.
//
// For a custom management of user use HandleClientAdd and HandleClientRemove
// on Server.
//
type ClientManager struct {
	sync.Mutex

	// all connections.
	all []int

	// conns contains a one-to-many mapping between user ID and their
	// connections.
	conns map[uint64][]int

	// ctx contains a one-to-one mapping between a socket and its context.
	// The context value is a result from successful authentication,
	// HandleAuth on Server.
	ctx map[int]context.Context

	// frame contains a one-to-one mapping between a socket connection
	// and a continuous frame.
	frame map[int]*Frame
}

//
// newClientManager create and initialize new user sockets.
//
func newClientManager() *ClientManager {
	return &ClientManager{
		conns: make(map[uint64][]int),
		ctx:   make(map[int]context.Context),
		frame: make(map[int]*Frame),
	}
}

//
// All return a copy of all client connections.
//
func (cls *ClientManager) All() (conns []int) {
	cls.Lock()
	conns = make([]int, len(cls.all))
	copy(conns, cls.all)
	cls.Unlock()
	return
}

//
// Conns return list of connections by user ID.
//
// Each user may have more than one connection (e.g. from Android, iOS, or
// web). By knowing which connections that user have, implementor of websocket
// server can broadcast a message to all connections.
//
func (cls *ClientManager) Conns(uid uint64) (conns []int) {
	cls.Lock()
	conns = cls.conns[uid]
	cls.Unlock()
	return
}

//
// Context return the client context.
//
func (cls *ClientManager) Context(conn int) (ctx context.Context) {
	cls.Lock()
	ctx = cls.ctx[conn]
	cls.Unlock()
	return
}

//
// Frame return continuous frame on a client connection.
//
func (cls *ClientManager) Frame(conn int) (frame *Frame, ok bool) {
	cls.Lock()
	frame, ok = cls.frame[conn]
	cls.Unlock()
	return
}

//
// SetFrame set the continuous frame on client connection.  If frame is nil,
// it will delete the stored frame in connection.
//
func (cls *ClientManager) SetFrame(conn int, frame *Frame) {
	cls.Lock()
	if frame == nil {
		delete(cls.frame, conn)
	} else {
		cls.frame[conn] = frame
	}
	cls.Unlock()
}

//
// add new socket connection to user ID with its context.
//
func (cls *ClientManager) add(ctx context.Context, conn int) {
	// Delete the previous socket reference on other user ID.
	cls.remove(conn)
	uid := ctx.Value(CtxKeyUID).(uint64)

	cls.Lock()

	if !ints.IsExist(cls.all, conn) {
		cls.all = append(cls.all, conn)
	}

	cls.ctx[conn] = ctx

	if uid > 0 {
		conns, ok := cls.conns[uid]
		if !ok {
			conns = make([]int, 0, 1)
		}
		conns = append(conns, conn)

		cls.conns[uid] = conns
	}

	cls.Unlock()
}

//
// remove a connection from list of clients.
//
func (cls *ClientManager) remove(conn int) {
	cls.Lock()

	delete(cls.frame, conn)
	ints.Remove(cls.all, conn)

	ctx, ok := cls.ctx[conn]
	if ok {
		uid := ctx.Value(CtxKeyUID).(uint64)
		delete(cls.ctx, conn)

		conns, ok := cls.conns[uid]
		if ok {
			conns, _ = ints.Remove(conns, conn)

			if len(conns) == 0 {
				delete(cls.conns, uid)
			} else {
				cls.conns[uid] = conns
			}
		}
	}

	cls.Unlock()
}
