/*
	gobal - http.go

	Our Connection class, provides very basic functionality for receiving and
	sending data.

	Copyright (c) 2012-2013 by authors and contributors.
*/

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"path"
	"strings"
)

type HttpConnection struct {
	conn    net.Conn
	BReader *bufio.Reader
	BWriter *bufio.Writer
	Service *Service
}

//////////////////////////////////////////////////////////////////////////////
// HTTP helpers
//////////////////////////////////////////////////////////////////////////////

func StatusForCode(status int) string {
	switch status {
	case 200:
		return "OK"
	case 500:
		return "Internal Server Error"
	default:
		return "Unknown"
	}
}

func CleanPath(root, uri string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		// Yes, this is totally fatal. We should never get here unless we are
		// serving a request without a document root.
		log.Fatal("docroot MUST be set")
	}

	// Try to ensure that the user hasn't escaped from our docroot by ensuring
	// that the docroot is still a prefix. NOTE: Join calls Clean, which will
	// expand .. etc, so this is safe.
	npath := path.Join(root, strings.TrimSpace(uri))
	if !strings.HasPrefix(npath, root) {
		return "", errors.New("URI escaped from root")
	}
	return npath, nil
}

// HttpSimpleResponse puts together a very simple, very boring response.
func HttpSimpleResponse(req *http.Request, status int,
	body string) *http.Response {
	return &http.Response{
		Request:       req,
		Status:        StatusForCode(status),
		StatusCode:    status,
		ContentLength: int64(len(body)),
		Body:          ioutil.NopCloser(strings.NewReader(body)),
	}
}

// HttpErrorResponse is a wrapper for building an http.Response struct for a
// simple error message.
func HttpErrorResponse(req *http.Request, err error) *http.Response {
	return HttpSimpleResponse(req, 500, fmt.Sprintf("Failure: %s", err))
}

//////////////////////////////////////////////////////////////////////////////
// HttpConnection base implementation
//////////////////////////////////////////////////////////////////////////////

// HttpAcceptor takes a TcpConnection that refers to a user, a Service that
// accepted it, and the ipport for where the connection came in on.
func HttpAcceptor(conn net.Conn, svc *Service, ipport string) error {
	hconn := &HttpConnection{
		conn:    conn,
		BReader: bufio.NewReader(conn),
		BWriter: bufio.NewWriter(conn),
		Service: svc,
	}
	go hconn.pump()
	return nil
}

// pump is the internal method for pulling requests out of a connection. This
// is a simple implementation that does not support fancy HTTP/1.1 features.
func (h *HttpConnection) pump() {
	defer h.Close()

	for {
		req, err := h.ReadRequest()
		if err != nil {
			log.Error("clientPumpHttp: %s", err)
			return
		}

		// We get here when we've received the headers. It could have body that
		// we are still waiting on, but that's OK. The included Body member
		// is a ReadCloser that will fetch only exactly what is in the body.
		// We build a channel for the service to pass the response back to us,
		// and then block on that. (We can't pipeline.)
		rchan := make(chan *http.Response, 1)

		if err := h.Service.HandleRequest(h, req, rchan); err != nil {
			h.WriteResponse(HttpErrorResponse(req, err))
			return
		}

		resp := <-rchan

		// TODO: Fix up response with keepalive, something like:
		//   h.setupKeepalive(req, resp)

		if err := h.WriteResponse(resp); err != nil {
			// We don't know what state the connection is in. Maybe we wrote
			// half a response already? Log the error then abort this conn.
			log.Error("pump failed: %s", err)
			return
		}

		// TODO: Close the connection if we're not doing keepalive.
	}
}

// ReadRequest reads in an http.Request object from the underlying transport.
func (h *HttpConnection) ReadRequest() (*http.Request, error) {
	req, err := http.ReadRequest(h.BReader)
	if err != nil {
		return nil, err
	}
	return req, nil
}

// WriteResponse takes an http.Response object and writes it out to the
// underlying transport, returning any errors.
func (h *HttpConnection) WriteResponse(r *http.Response) error {
	if err := r.Write(h.BWriter); err != nil {
		return err
	}
	return nil
}

// Close discards an HTTP connection. This is a hard close and just drops the
// underlying TCP transport immediately.
func (h *HttpConnection) Close() error {
	return h.conn.Close()
}
