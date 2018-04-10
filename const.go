package main

import "errors"

var ErrAlreadyListening = errors.New("Already listening")
var ErrAlreadyRegistered = errors.New("Already registered")
var ErrUnknownKey = errors.New("Unknown key")
var ErrUnknownIdentifier = errors.New("Unknown identifier")
var ErrDifferentToken = errors.New("Different token")
