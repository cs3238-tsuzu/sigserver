// Copyright (c) 2018 tsuzu
//
// This software is released under the MIT License.
// https://opensource.org/licenses/MIT

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	res, err := client.Register(context.Background(), &api.RegisterParameters{
		Key:       key,
		AuthToken: "", // Not implemented
	})

	if err != nil {
		panic(err)
	}
	fmt.Println("result: ", res)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	defer client.Unregister(context.Background(), &api.ListenParameters{
		Key:   key,
		Token: res.Token,
	})

	go func() {
		ch := make(chan os.Signal, 10)

		signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)

		<-ch

		cancel()
	}()

	listenClient, err := client.Listen(ctx, &api.ListenParameters{
		Key:   key,
		Token: res.Token,
	})

	if err != nil {
		panic(err)
	}

	for {
		accepted, err := listenClient.Recv()

		if err != nil {
			log.Println("accept error:", err)

			return
		}

		log.Println("accepted: ", accepted)

		func() {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			comm, err := client.Communicate(ctx)

			if err != nil {
				log.Println("communicate error:", err)

				return
			}

			defer comm.CloseSend()

			if err := comm.Send(&api.CommunicateParameters{
				Param: &api.CommunicateParameters_Identifier{
					Identifier: accepted.Identifier,
				},
			}); err != nil {
				log.Println("identifier authentication error:", err)

				return
			}

			res, err := comm.Recv()

			if err != nil {
				log.Println("recv error:", err)

				return
			}

			log.Println("recv:", res)

			if err := comm.Send(&api.CommunicateParameters{
				Param: &api.CommunicateParameters_Message{
					Message: res.Message,
				},
			}); err != nil {
				log.Println("send error:", err)

				return
			}

			log.Println("connection closed")
		}()
	}

}
