// Copyright (c) 2018 tsuzu
//
// This software is released under the MIT License.
// https://opensource.org/licenses/MIT

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cs3238-tsuzu/sigserver/api"

	"google.golang.org/grpc"
)

func main() {
	key := "test_key"
	address := "localhost:8080"

	if addr := os.Getenv("ADDRESS"); addr != "" {
		address = addr
	}

	conn, err := grpc.Dial(address, grpc.WithInsecure())

	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := api.NewListenerClient(conn)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go func() {
		ch := make(chan os.Signal, 10)

		signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)

		<-ch

		log.Println("interrupted")
		cancel()
	}()

	res, err := client.Connect(ctx, &api.ConnectParameters{
		Key: key,
	})

	if err != nil {
		panic(err)
	}

	comm, err := client.Communicate(ctx)

	if err != nil {
		log.Println("communicate error:", err)

		return
	}

	if err := comm.Send(&api.CommunicateParameters{
		Param: &api.CommunicateParameters_Identifier{
			Identifier: res.Identifier,
		},
	}); err != nil {
		log.Println("identifier authentication error:", err)

		return
	}

	if err := comm.Send(&api.CommunicateParameters{
		Param: &api.CommunicateParameters_Message{
			Message: "Hello, world!",
		},
	}); err != nil {
		log.Println("identifier authentication error:", err)

		return
	}

	recved, err := comm.Recv()

	if err != nil {
		log.Println("recv error:", err)

		return
	}

	log.Println("recv:", recved)

	if recved.Message == "Hello, world!" {
		log.Println("correct")
	} else {
		log.Println("incorrect")
	}

	comm.CloseSend()

}
