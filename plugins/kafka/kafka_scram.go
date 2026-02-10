package main

import (
	"crypto/sha256"
	"crypto/sha512"
	"hash"

	"github.com/xdg-go/scram"
)

// SHA256 is the hash generator function for SCRAM-SHA-256
var SHA256 scram.HashGeneratorFcn = func() hash.Hash { return sha256.New() }

// SHA512 is the hash generator function for SCRAM-SHA-512
var SHA512 scram.HashGeneratorFcn = func() hash.Hash { return sha512.New() }

// XDGSCRAMClient implements the sarama.SCRAMClient interface
type XDGSCRAMClient struct {
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

// Begin starts the SCRAM exchange
func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	client, err := x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = client.NewConversation()
	return nil
}

// Step advances the SCRAM exchange
func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	return x.ClientConversation.Step(challenge)
}

// Done returns true if the SCRAM exchange is complete
func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
