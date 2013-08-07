/*
	gobal - service.go

	Defines a Service object, which is a distinct set of configuration. A given
	Service can listen on several ports, but always handles requests in the
	same way.

	Copyright (c) 2012-2013 by authors and contributors.
*/

package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
)

type ServiceRole int

const (
	ROLE_WEBSERVER ServiceRole = iota
	ROLE_PROXY     ServiceRole = iota
	ROLE_MANAGE    ServiceRole = iota
)

// NOTE: We don't use pointers to this struct typically, since the contents of
// the struct are just a few pointers. Just copy by value.
type ServiceRequest struct {
	client  *HttpConnection
	request *http.Request
	rchan   chan *http.Response
}

type ServiceListener struct {
	Listener *TcpListener
	Acceptor AcceptorFunc
}

type Service struct {
	// Base service data
	Name      string
	Enabled   bool
	Role      ServiceRole
	Listeners map[string]*ServiceListener

	// ROLE_WEBSERVER related
	DocRoot string

	// ROLE_PROXY related
	Pool         *Pool
	requestQueue chan ServiceRequest
}

var serviceLock sync.Mutex
var services map[string]*Service = make(map[string]*Service)
var serviceDefaults map[string]string = make(map[string]string)

//////////////////////////////////////////////////////////////////////////////
// Service methods
//////////////////////////////////////////////////////////////////////////////

func ServiceDefault(key, value string) {
	if value == "" {
		delete(serviceDefaults, key)
	} else {
		serviceDefaults[key] = value
	}
}

//////////////////////////////////////////////////////////////////////////////
// Service base implementation
//////////////////////////////////////////////////////////////////////////////

// NewService creates a service with a given name. It is an error to create two
// services with the same name. By default, a service does nothing useful until
// it has been configured.
func NewService(name string) (*Service, error) {
	serviceLock.Lock()
	defer serviceLock.Unlock()

	if _, ok := services[name]; ok {
		return nil, errors.New(fmt.Sprintf("service '%s' already exists", name))
	}

	services[name] = &Service{
		Name:      name,
		Enabled:   false,
		Role:      ROLE_WEBSERVER,
		Listeners: make(map[string]*ServiceListener),
	}

	go services[name].requestPump()

	return services[name], nil
}

// serveFile takes as input a request from a client and then does something
// useful with that request. This is only called on ROLE_WEBSERVER services.
func (s *Service) serveFile(req ServiceRequest) {
	filepath, err := CleanPath(s.DocRoot, req.request.RequestURI)
	if err != nil {
		req.rchan <- HttpErrorResponse(req.request, err)
		return
	}

	fi, err := os.Stat(filepath)
	if err != nil {
		req.rchan <- HttpErrorResponse(req.request, err)
		return
	}

	// If it's a directory, try appending index.html
	if fi.IsDir() {
		filepath = path.Join(filepath, "index.html")
	}

	f, err := os.Open(filepath)
	if err != nil {
		req.rchan <- HttpErrorResponse(req.request, err)
		return
	}

	// TODO: Don't read the file into main memory. Splice it, send this
	// reading filehandle to the user's writing filehandle...
	rd, err := ioutil.ReadAll(f)
	if err != nil {
		req.rchan <- HttpErrorResponse(req.request, err)
		return
	}

	req.rchan <- HttpSimpleResponse(req.request, 200, string(rd))
}

// requestPump is a goroutine. It takes incoming requests and does something
// useful with them. This might be serving them (in the case of webserver) or
// it could match them up with backends.
//
// This is designed for high volume sytems. In the very simple case, it'll be
// inefficient because we won't have backends ready to go when a request comes
// in, since they will have expired. In a busy site, though, gobal will have
// a backend queue primed and ready.
func (s *Service) requestPump() {
	for {
		req := <-s.requestQueue

		// FIXME: Sanity check: is the client still around?
		// if req.client.alive ...

		if s.Role == ROLE_WEBSERVER {
			go s.serveFile(req)
			return
		} else if s.Role != ROLE_PROXY {
			log.Error("unexpected role in Service.requestPump")
			req.rchan <- HttpErrorResponse(req.request,
				errors.New("Invalid service type"))
			return
		}

		// At this point we're guaranteed to be a ROLE_PROXY. Fetch a backend,
		// which might block a bit.
		be := s.Pool.GetBackend()
	}
}

// Enable is called when we're done doing setup and need to activate things such
// as our listeners.
func (s *Service) Enable() error {
	for ipport, lstnr := range s.Listeners {
		if lstnr.Listener != nil {
			continue
		}

		// Instantiates a TcpListener goroutine to handle accepting connections
		// on this particular ipport.
		tlstnr, err := ListenTcp(ipport, lstnr.Acceptor)
		if err != nil {
			log.Error("failed to listen on %s: %s", ipport, err)
			continue
		}
		s.Listeners[ipport].Listener = tlstnr
	}
	s.Enabled = true
	return nil
}

// setListen takes a new listen string and handles it.
func (s *Service) setListen(value string, acceptor AcceptorFunc) error {
	if len(s.Listeners) > 0 {
		log.Warn("changing existing listeners on service %s", s.Name)
	}

	// TODO: don't close all listeners if we're just adding to the list, only
	// close what we need to.
	for ipport, lstnr := range s.Listeners {
		if lstnr != nil {
			lstnr.Listener.Close()
		}
		delete(s.Listeners, ipport)
	}

	for _, ipport := range strings.Split(value, ",") {
		ipport = strings.TrimSpace(ipport)
		log.Debug("creating ServiceListener on %s", ipport)
		s.Listeners[ipport] = &ServiceListener{
			Listener: nil,
			Acceptor: acceptor,
		}
	}

	if s.Enabled {
		s.Enable() // Causes the above to start listening.
	}
	return nil
}

// Accept takes an incoming connection from a listener and then passes it down
// to the appropriate acceptor for whatever our role is.
func (s *Service) Accept(conn net.Conn, ipport string) error {
	switch s.Role {
	case ROLE_MANAGE:
		return TcpAcceptor(conn, s, ipport)
	case ROLE_PROXY, ROLE_WEBSERVER:
		return HttpAcceptor(conn, s, ipport)
	default:
		log.Fatal("unknown role in accept")
	}
	return errors.New("Accept fell through!")
}

// Set configures our service. This is generally called by the configuration
// engine, although there's no particular constraint on that.
func (s *Service) Set(key, value string) error {
	switch key {
	case "listen":
		return s.setListen(value, s.Accept)
	case "role":
		switch value {
		case "web_server":
			s.Role = ROLE_WEBSERVER
		case "management":
			s.Role = ROLE_MANAGE
		case "reverse_proxy":
			s.Role = ROLE_PROXY
		default:
			return errors.New(fmt.Sprintf("invalid role '%s'", value))
		}
	case "docroot":
		value = path.Clean(strings.TrimSpace(value))
		fi, err := os.Stat(value)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			return errors.New(fmt.Sprintf("docroot: %s is not a directory",
				value))
		}
		s.DocRoot = value
	case "pool":
		pool, ok := pools[value]
		if !ok {
			return errors.New(fmt.Sprintf("pool '%s' not found", value))
		}
		s.Pool = pool
	default:
		log.Error("unknown SET %s.%s = %s", s.Name, key, value)
	}
	return nil
}

// HandleRequest is a method that takes in an HttpConnection and an http.Request
// and puts it on our queue to be handled. NOTE: If you are going to return an
// error from this function, you MUST NOT write to the connection. Errors are
// automatically sent to the user.
func (s *Service) HandleRequest(conn *HttpConnection, req *http.Request,
	rchan chan *http.Response) error {

	// For now, all requests are just enqueued. We could do some work in this
	// function if we wanted to support blacklisting, delaying requests, or some
	// other stuff?

	// If this blocks, then all we're doing is gumming up the pump for the
	// client connection. That's OK for HTTP.
	s.requestQueue <- ServiceRequest{
		client:  conn,
		request: req,
		rchan:   rchan,
	}
	return nil
}
