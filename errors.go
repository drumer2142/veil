package main

import "errors"

var (
	errInvalidURL   = errors.New("invalid url")
	errNotFound     = errors.New("not found")
	errBadRequest   = errors.New("bad request")
	errPayloadLimit = errors.New("payload too large")
)
