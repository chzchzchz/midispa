package main

type Policy interface {
	handle(msg []byte) bool
}
