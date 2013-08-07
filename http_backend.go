/*
	gobal - http_backend.go

	Provides a connection to a backend HTTP server.

	Copyright (c) 2013 by authors and contributors.
*/

package main

import ()

type HttpBackendConnection struct {
	Conn   *TcpConnection
	Client *HttpConnection
}

//////////////////////////////////////////////////////////////////////////////
// HttpBackendConnection base implementation
//////////////////////////////////////////////////////////////////////////////

// HttpBackend creates a connection to a backend, setting up the various
// data structures that we need and initiating the connection.
func MakeHttpBackend(be *Backend) error {
	conn, err := MakeTcpConnection(be.Ipport)
	if err != nil {
		return err
	}

	hconn := HttpBackendConnection{
		Conn:   conn,
		Client: nil,
	}

	return nil
}

// Close discards an HTTP connection. This is a hard close and just drops the
// underlying TCP transport immediately.
func (h *HttpBackendConnection) Close() error {
	if h.Client != nil {
		if err := h.Client.Close(); err != nil {
			return err
		}
	}
	if err := h.Conn.Close(); err != nil {
		return err
	}
	return nil
}
