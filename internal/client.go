package internal

import (
	"context"
	"net"
	"net/http"
	"time"
)

type Context struct {
	ctx    context.Context
	cancel context.CancelFunc
}

type Client struct {
	AgentName string
	BotName   string
	Client    *http.Client
	Context   Context
}

func NewClient() *Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          100,
		MaxConnsPerHost:       10,
		IdleConnTimeout:       90 * time.Second,
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	botName := "Grawl"
	myAgent := botName + "/1.0 (trstnalharrish@gmail.com)"
	ctx, cancel := context.WithCancel(context.Background())

	context := Context{
		ctx:    ctx,
		cancel: cancel,
	}

	return &Client{
		AgentName: myAgent,
		BotName:   botName,
		Client:    client,
		Context:   context,
	}
}
