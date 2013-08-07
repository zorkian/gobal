/*
	gobal - conn.go

	Our Connection class, provides very basic functionality for receiving and
	sending data.

	Copyright (c) 2012-2013 by authors and contributors.
*/

package main

import (
	"bufio"
	"errors"
	"net"
	"time"
)

// gobal is designed for HTTP, hence it uses TCP connections mostly.
type TcpConnection struct {
	Conn    net.Conn
	BReader *bufio.Reader
	BWriter *bufio.Writer
	alive   bool
}

//////////////////////////////////////////////////////////////////////////////
// TcpConnection base implementation
//////////////////////////////////////////////////////////////////////////////

func TcpAcceptor(conn net.Conn, svc *Service, ipport string) error {
	wrap, err := WrapTcpConnection(conn)
	if err != nil {
		return err
	}

	go wrap.pump()
	return nil
}

// MakeTcpConnection constructs a new TcpConnection object from a given address.
// This establishes an outgoing connection.
func MakeTcpConnection(ipport string) (*TcpConnection, error) {
	conn, err := net.DialTimeout("tcp", ipport, 3*time.Second)
	if err != nil {
		return nil, err
	}

	return WrapTcpConnection(conn)
}

// WrapTcpConnection takes a bare net.TCPConn and wraps it up in a TcpConnection
// after constructing some readers and writers for us to use.
func WrapTcpConnection(conn net.Conn) (*TcpConnection, error) {
	c := &TcpConnection{
		Conn:    conn,
		BReader: bufio.NewReader(conn),
		BWriter: bufio.NewWriter(conn),
		alive:   true,
	}

	return c, nil
}

// pump is called for bare TcpConnection line based protocols. These are then
// treated as commands and passed to the service to handle.
func (c *TcpConnection) pump() {
	defer c.Close()

	for {
		ln, err := c.ReadLine()
		if err != nil {
			return
		}

		// Handle an administration command of some sort.
		log.Debug("received: %s", ln)
	}
}

// Close does the obvious to our connection. This handles any cleanup
func (c *TcpConnection) Close() error {
	if !c.alive {
		return errors.New("TcpConnection already closed")
	}
	c.alive = false
	err := c.BWriter.Flush()
	if err != nil {
		log.Error("failed flush: %s", err)
	}
	log.Debug("TcpConnection: closing")
	return c.Conn.Close()
}

// ReadLine reads an input line. This is only useful in TCP mode, if you are
// expecting HTTP requests, don't use this method. It will put things in a bad
// state.
func (c *TcpConnection) ReadLine() (string, error) {
	return c.BReader.ReadString('\n')
}

// WriteLine sends a line of output.
func (c *TcpConnection) WriteLine(line string) error {
	_, err := c.BWriter.WriteString(line + "\n")
	return err
}
