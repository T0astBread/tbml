package com

type MsgTypeIn string

const (
	MsgTypeInInitControlSocketPath MsgTypeIn = "init-control-socket-path"
	MsgTypeInTBML                  MsgTypeIn = "tbml"
)

type MsgIn struct {
	Type MsgTypeIn   `json:"type"`
	Data interface{} `json:"data"`
}

type MsgTypeOut string

const (
	MsgTypeOutConnectorError MsgTypeOut = "connector-error"
	MsgTypeOutConnectorLog   MsgTypeOut = "connector-log"
	MsgTypeOutTBML           MsgTypeOut = "tbml"
)

type MsgOut struct {
	Type MsgTypeOut  `json:"type"`
	Data interface{} `json:"data"`
}
