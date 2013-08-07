/*
	gobal - spdy.go

	Our SpdyConnection class, provides the implementation of SPDY.

	Copyright (c) 2013 by authors and contributors.
*/

package main

import (
	"errors"
)

// SpdySession is the equivalent of a connection to a user. This will contain
// many streams that are themselves used for making requests.
type SpdySession struct {
	Conn      *TcpConnection
	alive     bool
	bytesLeft uint32
}

//////////////////////////////////////////////////////////////////////////////
// SpdyConnection base implementation
//////////////////////////////////////////////////////////////////////////////

func WrapSpdySession(conn *TcpConnection) (*SpdySession, error) {
	c := &SpdySession{
		Conn:  conn,
		alive: true,
	}

	return c, nil
}

// Close on a SPDY session. This should be gentle and tell the user that we're
// cutting them off.
func (c *SpdySession) Close() error {
	if !c.alive {
		return errors.New("SpdySession already closed")
	}
	c.alive = false

	// TODO: send GOAWAY frame and close gently.

	log.Debug("SpdySession closing")
	return c.Conn.Close()
}
