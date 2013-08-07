/*
	gobal - conn.go

	Our Connection class, provides very basic functionality for receiving and
	sending data.

	Copyright (c) 2012-2013 by authors and contributors.
*/

package main

import (
	"net"
)

// AcceptorFunc is someone who can take a connection and do something useful
// with it. This should be implemented by people who want to hold listeners.
type AcceptorFunc func(net.Conn, string) error

// gobal is designed for HTTP, hence it uses TCP connections mostly. This class
type TcpListener struct {
	alive  bool
	ipport string
	socket *net.TCPListener
}

//////////////////////////////////////////////////////////////////////////////
// TcpListener implementation
//////////////////////////////////////////////////////////////////////////////

// ListenTcp takes an IP and port and constructs a listener on the given combo.
// Returns an object that accepts connections and passes them back, or an error.
func ListenTcp(ipport string, acceptor AcceptorFunc) (*TcpListener, error) {
	addr, err := net.ResolveTCPAddr("tcp4", ipport)
	if err != nil {
		return nil, err
	}

	socket, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		return nil, err
	}
	log.Debug("listening on tcp4 %s...", addr)

	l := &TcpListener{
		alive:  true,
		socket: socket,
		ipport: ipport,
	}

	go l.acceptLoop(acceptor)
	return l, nil
}

// acceptLoop is an internal worker that accepts connections on a given
// TcpListener and sends the connections down
func (l *TcpListener) acceptLoop(acceptor AcceptorFunc) {
	for {
		conn, err := l.socket.AcceptTCP()
		log.Debug("acceptLoop(%s): new connection", l.socket.Addr())
		if err != nil {
			log.Error("acceptLoop(%s): %s", l.socket.Addr(), err)
			return
		}

		// If this fails, oh well. Not our problem. Keep accepting and log it
		// so that someone will fix things.
		if err = acceptor(conn, l.ipport); err != nil {
			conn.Close()
			log.Error("acceptLoop(%s): %s", l.socket.Addr(), err)
		}
	}
}

// Close terminates an active listener, telling it to stop listening.
func (l *TcpListener) Close() error {
	log.Debug("Close(%s): closing", l.socket.Addr())
	l.alive = false
	l.socket.Close()
	return nil
}
