// Copyright 2018-2019 opcua authors. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be
// found in the LICENSE file.

package main

import (
	"crypto/rsa"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/debug"
	"github.com/gopcua/opcua/ua"
	"github.com/gopcua/opcua/uasc"
)

func main() {
	var (
		endpoint = flag.String("endpoint", "opc.tcp://localhost:4840", "OPC UA Endpoint URL")
		certfile = flag.String("cert", "cert.pem", "Path to certificate file")
		keyfile  = flag.String("key", "key.pem", "Path to PEM Private Key file")
		gencert  = flag.Bool("gen-cert", false, "Generate a new certificate")
		policy   = flag.String("sec-policy", "", "Security Policy URL or one of None, Basic128Rsa15, Basic256, Basic256Sha256")
		mode     = flag.String("sec-mode", "", "Security Mode: one of None, Sign, SignAndEncrypt")
		auth     = flag.String("auth-mode", "Anonymous", "Authentication Mode: one of Anonymous, UserName, Certificate")
		appuri   = flag.String("app-uri", "urn:gopcua:client", "Application URI")
		list     = flag.Bool("list", false, "List the policies supported by the endpoint and exit")
	)
	flag.BoolVar(&debug.Enable, "debug", false, "enable debug logging")
	flag.Parse()
	log.SetFlags(0)

	// Get a list of the endpoints for our target server
	endpoints, err := opcua.GetEndpoints(*endpoint)
	if err != nil {
		log.Fatal(err)
	}

	if *list {
		printEndpointOptions(*endpoint, endpoints)
		return
	}

	// Find the endpoint recommended by the server (highest SecurityMode+SecurityLevel)
	var serverEndpoint *ua.EndpointDescription
	for _, e := range endpoints {
		if serverEndpoint == nil || (e.SecurityMode >= serverEndpoint.SecurityMode && e.SecurityLevel >= serverEndpoint.SecurityLevel) {
			serverEndpoint = e
		}
	}

	var secPolicy string
	switch {
	case *policy != "" && !strings.HasPrefix(*policy, "http://"):
		secPolicy = "http://opcfoundation.org/UA/SecurityPolicy#" + *policy
	case strings.HasPrefix(*policy, "http://"):
		secPolicy = *policy
	default:
		secPolicy = serverEndpoint.SecurityPolicyURI
	}

	var secMode ua.MessageSecurityMode
	switch *mode {
	case "None":
		secMode = ua.MessageSecurityModeNone
	case "Sign":
		secMode = ua.MessageSecurityModeSign
	case "SignAndEncrypt":
		secMode = ua.MessageSecurityModeSignAndEncrypt
	default:
		secMode = serverEndpoint.SecurityMode
	}

	// Allow input of only one of sec-mode,sec-policy when choosing 'None'
	if secMode == ua.MessageSecurityModeNone || secPolicy == "http://opcfoundation.org/UA/SecurityPolicy#None" {
		secMode = ua.MessageSecurityModeNone
		secPolicy = "http://opcfoundation.org/UA/SecurityPolicy#None"
	}

	var authMode ua.UserTokenType
	var authOption opcua.Option
	switch *auth {
	case "Anonymous":
		authMode = ua.UserTokenTypeAnonymous
		authOption = opcua.AuthAnonymous()
	case "UserName":
		authMode = ua.UserTokenTypeUserName
		authOption = opcua.AuthUsername("", "") // todo
	case "Certificate":
		authMode = ua.UserTokenTypeCertificate
		authOption = opcua.AuthCertificate([]byte(nil)) // todo
	case "IssuedToken":
		authMode = ua.UserTokenTypeIssuedToken
		authOption = opcua.AuthIssuedToken([]byte(nil)) // todo
	default:
		log.Printf("unknown auth-mode, defaulting to Anonymous")
		authMode = ua.UserTokenTypeAnonymous
		authOption = opcua.AuthAnonymous()
	}

	// Check that the selected endpoint is a valid combo
	var valid bool
outer:
	for _, e := range endpoints {
		if e.SecurityMode == secMode && e.SecurityPolicyURI == secPolicy {
			for _, t := range e.UserIdentityTokens {
				if t.TokenType == authMode {
					valid = true
					break outer
				}
			}
		}
	}
	if !valid {
		fmt.Printf("server does not support an endpoint with security : %s , %s\n", *policy, *mode)
		printEndpointOptions(*endpoint, endpoints)
		return
	}

	opts := []opcua.Option{
		opcua.SecurityFromEndpoint(serverEndpoint, authMode),
		opcua.SecurityPolicy(secPolicy),
		opcua.SecurityMode(secMode),
		authOption,
	}

	// ApplicationURI is automatically read from the cert so is not required if a cert if provided
	if *certfile == "" && !*gencert {
		opts = append(opts, opcua.ApplicationURI(*appuri))
	}

	switch secPolicy {
	case uasc.SecurityPolicyNone:
	default:
		if *gencert {
			generate_cert(*appuri, 2048, *certfile, *keyfile)
		}
		log.Printf("Loading cert/key from %s/%s", *certfile, *keyfile)
		cert, err := tls.LoadX509KeyPair(*certfile, *keyfile)
		if err != nil {
			log.Fatalf("Failed to load certificate: %s", err)
		}
		pk, ok := cert.PrivateKey.(*rsa.PrivateKey)
		if !ok {
			log.Fatalf("Invalid private key")
		}
		opts = append(opts, opcua.PrivateKey(pk), opcua.Certificate(cert.Certificate[0]))
	}

	// Finally, create our Client object
	c := opcua.NewClient(*endpoint, opts...)
	log.Printf("Connecting to %s, security mode: %s, %s \n", *endpoint, secPolicy, secMode)
	if err := c.Connect(); err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Use our connection (read the server's time)
	v, err := c.Node(ua.NewNumericNodeID(0, 2258)).Value()
	if err != nil {
		log.Fatal(err)
	}
	if v != nil {
		fmt.Printf("Server's Time | Conn 1 %s | ", v.Value)
	} else {
		log.Print("v == nil")
	}

	// Detach our session and try re-establish it on a different secure channel
	s, err := c.DetachSession()
	if err != nil {
		log.Fatalf("Error detaching session: %s", err)
	}

	d := opcua.NewClient(*endpoint, opts...)

	// Create a channel only and do not activate it automatically
	d.Dial()
	defer d.Close()

	// Activate the previous session on the new channel
	err = d.ActivateSession(s)
	if err != nil {
		log.Fatalf("Error reactivating session: %s", err)
	}

	// Read the time again to prove our session is still OK
	v, err = d.Node(ua.NewNumericNodeID(0, 2258)).Value()
	if err != nil {
		log.Fatal(err)
	}
	if v != nil {
		fmt.Printf("Conn 2: %s\n", v.Value)
	} else {
		log.Print("v == nil")
	}
}

func printEndpointOptions(endpoint string, endpoints []*ua.EndpointDescription) {
	log.Printf("Valid options for %s are:", endpoint)
	log.Printf("%22s , %-15s , (%s)\n", "sec-policy", "sec-mode", "auth-modes")
	for _, e := range endpoints {
		p := strings.TrimPrefix(e.SecurityPolicyURI, "http://opcfoundation.org/UA/SecurityPolicy#")
		m := strings.TrimPrefix(e.SecurityMode.String(), "MessageSecurityMode")
		var tt []string
		for _, t := range e.UserIdentityTokens {
			tok := strings.TrimPrefix(t.TokenType.String(), "UserTokenType")
			tt = append(tt, tok)
		}
		log.Printf("%22s , %-15s , (%s)", p, m, strings.Join(tt, ","))

	}
}